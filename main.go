package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
)

type DiffItem struct {
	Path string      `json:"path"`
	Type string      `json:"type"` // added, removed, modified
	A    interface{} `json:"a,omitempty"`
	B    interface{} `json:"b,omitempty"`
}

// ParseError describes a JSON parsing error with optional line/column
type ParseError struct {
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
}

func prettyJSON(b []byte) string {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		// return original if not valid JSON
		return string(b)
	}
	out, _ := json.MarshalIndent(v, "", "  ")
	return string(out)
}

// compute line and column (1-based) from byte offset (1-based)
func lineColFromOffset(b []byte, offset int64) (int, int) {
	if offset <= 0 {
		return 0, 0
	}
	if offset > int64(len(b)) {
		offset = int64(len(b))
	}
	// offset is 1-based
	idx := int(offset - 1)
	line := 1
	col := 1
	for i := 0; i < idx; i++ {
		if b[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

func parseErrorFor(b []byte) *ParseError {
	var v interface{}
	err := json.Unmarshal(b, &v)
	if err == nil {
		return nil
	}
	switch e := err.(type) {
	case *json.SyntaxError:
		ln, col := lineColFromOffset(b, e.Offset)
		return &ParseError{Message: e.Error(), Line: ln, Column: col}
	case *json.UnmarshalTypeError:
		ln, col := lineColFromOffset(b, e.Offset)
		return &ParseError{Message: e.Error(), Line: ln, Column: col}
	default:
		return &ParseError{Message: err.Error()}
	}
}

func joinPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func compareValues(path string, a, b interface{}, out *[]DiffItem) {
	// both nil
	if a == nil && b == nil {
		return
	}
	// a exists, b missing
	if b == nil && a != nil {
		*out = append(*out, DiffItem{Path: path, Type: "removed", A: a})
		return
	}
	// b exists, a missing
	if a == nil && b != nil {
		*out = append(*out, DiffItem{Path: path, Type: "added", B: b})
		return
	}

	switch av := a.(type) {
	case map[string]interface{}:
		bv, ok := b.(map[string]interface{})
		if !ok {
			// type changed
			*out = append(*out, DiffItem{Path: path, Type: "modified", A: a, B: b})
			return
		}
		// union of keys
		keys := map[string]struct{}{}
		for k := range av {
			keys[k] = struct{}{}
		}
		for k := range bv {
			keys[k] = struct{}{}
		}
		for k := range keys {
			pa := joinPath(path, k)
			var aa, bb interface{}
			if v, ok := av[k]; ok { aa = v }
			if v, ok := bv[k]; ok { bb = v }
			compareValues(pa, aa, bb, out)
		}
	case []interface{}:
		bv, ok := b.([]interface{})
		if !ok {
			*out = append(*out, DiffItem{Path: path, Type: "modified", A: a, B: b})
			return
		}
		min := len(av)
		if len(bv) < min {
			min = len(bv)
		}
		for i := 0; i < min; i++ {
			pa := path + "[" + strconv.Itoa(i) + "]"
			compareValues(pa, av[i], bv[i], out)
		}
		if len(av) > len(bv) {
			for i := min; i < len(av); i++ {
				pa := path + "[" + strconv.Itoa(i) + "]"
				*out = append(*out, DiffItem{Path: pa, Type: "removed", A: av[i]})
			}
		} else if len(bv) > len(av) {
			for i := min; i < len(bv); i++ {
				pa := path + "[" + strconv.Itoa(i) + "]"
				*out = append(*out, DiffItem{Path: pa, Type: "added", B: bv[i]})
			}
		}
	default:
		// primitives
		if !reflect.DeepEqual(a, b) {
			*out = append(*out, DiffItem{Path: path, Type: "modified", A: a, B: b})
		}
	}
}

func generateDiffList(a, b []byte) ([]DiffItem, error) {
	var av, bv interface{}
	if err := json.Unmarshal(a, &av); err != nil {
		// if invalid, treat as text
		if string(a) == string(b) {
			return nil, nil
		}
		return []DiffItem{{Path: "$", Type: "modified", A: string(a), B: string(b)}}, nil
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		if string(a) == string(b) {
			return nil, nil
		}
		return []DiffItem{{Path: "$", Type: "modified", A: av, B: string(b)}}, nil
	}
	var out []DiffItem
	compareValues("$", av, bv, &out)
	return out, nil
}

// countLeaves returns the number of leaf (primitive) nodes under v.
func countLeaves(v interface{}) int {
	if v == nil {
		return 0
	}
	switch vv := v.(type) {
	case map[string]interface{}:
		cnt := 0
		for _, val := range vv {
			cnt += countLeaves(val)
		}
		return cnt
	case []interface{}:
		cnt := 0
		for _, el := range vv {
			cnt += countLeaves(el)
		}
		return cnt
	default:
		return 1
	}
}

// countLeavesUnion counts leaf positions in the union of a and b (used to compute total compared fields).
func countLeavesUnion(a, b interface{}) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return countLeaves(b)
	}
	if b == nil {
		return countLeaves(a)
	}

	// both present
	switch av := a.(type) {
	case map[string]interface{}:
		if bv, ok := b.(map[string]interface{}); ok {
			keys := map[string]struct{}{}
			for k := range av {
				keys[k] = struct{}{}
			}
			for k := range bv {
				keys[k] = struct{}{}
			}
			cnt := 0
			for k := range keys {
				cnt += countLeavesUnion(av[k], bv[k])
			}
			return cnt
		}
	case []interface{}:
		if bv, ok := b.([]interface{}); ok {
			max := len(av)
			if len(bv) > max {
				max = len(bv)
			}
			cnt := 0
			for i := 0; i < max; i++ {
				var aa, bb interface{}
				if i < len(av) { aa = av[i] }
				if i < len(bv) { bb = bv[i] }
				cnt += countLeavesUnion(aa, bb)
			}
			return cnt
		}
	}
	// differing types or primitives: treat as single leaf
	return 1
}

func pathToJSONPointer(p string) string {
	// our paths look like $.a.b[2].c -> convert to /a/b/2/c
	if p == "$" || p == "$." || p == "" {
		return "" // pointer to whole document
	}
	// strip leading $. or $
	p = strings.TrimPrefix(p, "$")
	p = strings.TrimPrefix(p, ".")
	// replace [index] with /index and . with /
	p = strings.ReplaceAll(p, ".", "/")
	p = strings.ReplaceAll(p, "[", "/")
	p = strings.ReplaceAll(p, "]", "")
	// escape ~ and /
	p = strings.ReplaceAll(p, "~", "~0")
	p = strings.ReplaceAll(p, "/", "~1")
	return "/" + p
}

func doCompareTexts(aText, bText string) (map[string]interface{}, error) {
	a := []byte(aText)
	b := []byte(bText)

	prettyA := prettyJSON(a)
	prettyB := prettyJSON(b)

	errA := parseErrorFor(a)
	errB := parseErrorFor(b)

	// default placeholders
	asciiStr := ""
	patchStr := ""
	var dList []DiffItem
	identical := false

	// only compute diffs when both sides parsed OK
	if errA == nil && errB == nil {
		// decode with UseNumber to preserve numeric forms
		var av, bv interface{}
		decA := json.NewDecoder(bytes.NewReader(a))
		decA.UseNumber()
		if err := decA.Decode(&av); err != nil {
			return nil, err
		}
		decB := json.NewDecoder(bytes.NewReader(b))
		decB.UseNumber()
		if err := decB.Decode(&bv); err != nil {
			return nil, err
		}

		// detect semantic equality
		if reflect.DeepEqual(av, bv) {
			identical = true
			// leave asciiStr empty
		} else {
			// determine structured vs primitive
			isStructured := func(v interface{}) bool {
				switch v.(type) {
				case map[string]interface{}, []interface{}:
					return true
				default:
					return false
				}
			}

			if isStructured(av) && isStructured(bv) {
				// both structured; use gojsondiff
				differ := gojsondiff.New()
				delta, err := differ.Compare(a, b)
				if err != nil {
					return nil, err
				}
				var base interface{}
				_ = json.Unmarshal(a, &base)

				asciiCfg := formatter.AsciiFormatterConfig{ShowArrayIndex: true, Coloring: false}
				af := formatter.NewAsciiFormatter(base, asciiCfg)
				asciiStr, _ = af.Format(delta)

				deltaFmt := formatter.NewDeltaFormatter()
				patchStr, _ = deltaFmt.Format(delta)

				// structured diff list
				dList, _ = generateDiffList(a, b)
			} else {
				// mixed types or primitives: create a simple diff
				asciiStr = fmt.Sprintf("- %v\n+ %v\n", av, bv)
				dList = []DiffItem{{Path: "$", Type: "modified", A: av, B: bv}}
			}
		}
	}

	resp := map[string]interface{}{
		"prettyA":   prettyA,
		"prettyB":   prettyB,
		"asciiDiff": asciiStr,
		"patch":     patchStr,
		"diffList":  dList,
		"identical": identical,
	}

	// Build a simple RFC-6902 style patch from our structured diff list when possible
	if errA == nil && errB == nil {
		if len(dList) > 0 {
			ops := make([]map[string]interface{}, 0, len(dList))
			opsAnn := make([]map[string]interface{}, 0, len(dList))
			for _, di := range dList {
				p := pathToJSONPointer(di.Path)
				switch di.Type {
				case "added":
					ops = append(ops, map[string]interface{}{"op": "add", "path": p, "value": di.B})
					opsAnn = append(opsAnn, map[string]interface{}{"op": "add", "path": p, "value": di.B})
				case "removed":
					ops = append(ops, map[string]interface{}{"op": "remove", "path": p})
					opsAnn = append(opsAnn, map[string]interface{}{"op": "remove", "path": p, "old": di.A})
				case "modified":
					ops = append(ops, map[string]interface{}{"op": "replace", "path": p, "value": di.B})
					opsAnn = append(opsAnn, map[string]interface{}{"op": "replace", "path": p, "old": di.A, "value": di.B})
				}
			}
			// expose patches as structured objects (not strings) so clients can pretty-print and download
			resp["rfc6902PatchRFC"] = ops
			resp["rfc6902Patch"] = opsAnn
			resp["rfc6902PatchAnnotated"] = opsAnn
			// also include pretty-printed string forms for backward compatibility
			if pjBytes, err := json.MarshalIndent(ops, "", "  "); err == nil {
				resp["rfc6902PatchRFCString"] = string(pjBytes)
			} else {
				fmt.Println("marshal rfc6902 patch error:", err)
				resp["rfc6902PatchError"] = fmt.Sprintf("marshal patch error: %v", err)
			}
			if pjAnn, err := json.MarshalIndent(opsAnn, "", "  "); err == nil {
				resp["rfc6902PatchString"] = string(pjAnn)
			} else {
				fmt.Println("marshal annotated patch error:", err)
			}
		}
	}

	// compute statistics for diffs: totals and metrics
	if errA == nil && errB == nil {
		// compute counts
		added := 0
		removed := 0
		modified := 0
		for _, di := range dList {
			switch di.Type {
			case "added": added++
			case "removed": removed++
			case "modified": modified++
			}
		}
		totalDiffs := len(dList)

		// parse both into generic values to count total compared leaves
		var av, bv interface{}
		_ = json.Unmarshal(a, &av)
		_ = json.Unmarshal(b, &bv)
		totalCompared := countLeavesUnion(av, bv)
		if totalCompared == 0 { totalCompared = 0 }

		accuracy := 1.0
		if totalCompared > 0 {
			accuracy = float64(totalCompared-totalDiffs) / float64(totalCompared)
		}

		// precision: use standard formula precision = TP / (TP + FP)
		// where TP = totalCompared - totalDiffs (matching leaves),
		// and FP = added + modified (diffs counted as false positives per spec)
		TP := totalCompared - totalDiffs
		FP := added + modified
		precision := 1.0
		if TP+FP > 0 {
			precision = float64(TP) / float64(TP+FP)
		}

		stats := map[string]interface{}{
			"totalCompared": totalCompared,
			"totalDiffs":    totalDiffs,
			"added":         added,
			"removed":       removed,
			"modified":      modified,
			"accuracy":      accuracy,
			"precision":     precision,
		}
		resp["stats"] = stats
	}

	if errA != nil { resp["errorA"] = errA }
	if errB != nil { resp["errorB"] = errB }
	return resp, nil
}

func compareHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("--- /compare start ---")
	fmt.Println("remote:", r.RemoteAddr, "method:", r.Method, "content-type:", r.Header.Get("Content-Type"), "content-length:", r.ContentLength)

	// CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// recover from panic and return JSON error
	writeJSONError := func(status int, msg string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
		fmt.Println("compareHandler error:", status, msg)
	}
	defer func() {
		if rec := recover(); rec != nil {
			writeJSONError(http.StatusInternalServerError, fmt.Sprintf("panic: %v", rec))
			fmt.Println("panic stack:")
		}
	}()

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		fmt.Println("ParseMultipartForm error:", err)
		writeJSONError(http.StatusBadRequest, "parse error: "+err.Error())
		return
	}

	readPart := func(key string) string {
		f, _, err := r.FormFile(key)
		if err == nil && f != nil {
			defer f.Close()
			b, err := io.ReadAll(f)
			if err != nil {
				fmt.Println("readPart read error:", key, err)
				return ""
			}
			fmt.Println("readPart: read file", key, "len=", len(b))
			return string(b)
		}
		if err != nil && err != http.ErrMissingFile {
			// log but continue to try FormValue
			fmt.Println("readPart FormFile error for", key, err)
		}
		if v := r.FormValue(key); v != "" {
			fmt.Println("readPart: form value", key, "len=", len(v))
			return v
		}
		return ""
	}

	a := readPart("fileA")
	b := readPart("fileB")
	fmt.Println("payload lengths: A=", len(a), "B=", len(b))

	res, err := doCompareTexts(a, b)
	if err != nil {
		fmt.Println("doCompareTexts error:", err)
		writeJSONError(http.StatusInternalServerError, "compare error: "+err.Error())
		return
	}

	// if server-side parse errors detected, return 400 with error details
	if _, ok := res["errorA"]; ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(res)
		fmt.Println("--- /compare parse error (A) ---")
		return
	}
	if _, ok := res["errorB"]; ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(res)
		fmt.Println("--- /compare parse error (B) ---")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(res)
	fmt.Println("--- /compare ok ---")
}

func main() {
	fs := http.FileServer(http.Dir("./frontend"))
	http.Handle("/", fs)
	http.HandleFunc("/compare", compareHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}
	fmt.Println("listening on http://localhost:" + port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
