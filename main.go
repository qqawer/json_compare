package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strconv"

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

func compareHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "parse error", http.StatusBadRequest)
		return
	}
	readPart := func(key string) []byte {
		f, _, err := r.FormFile(key)
		if err == nil && f != nil {
			defer f.Close()
			b, _ := io.ReadAll(f)
			return b
		}
		// try plain field
		if v := r.FormValue(key); v != "" {
			return []byte(v)
		}
		return []byte("{}")
	}
	a := readPart("fileA")
	b := readPart("fileB")

	prettyA := prettyJSON(a)
	prettyB := prettyJSON(b)

	// detect parse errors
	errA := parseErrorFor(a)
	errB := parseErrorFor(b)

	// default placeholders
	asciiStr := ""
	patchStr := ""
	var dList []DiffItem

	// only compute diffs when both sides parsed OK
	if errA == nil && errB == nil {
		differ := gojsondiff.New()
		delta, err := differ.Compare(a, b)
		if err != nil {
			http.Error(w, "compare error: "+err.Error(), http.StatusInternalServerError)
			return
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
	}

	resp := map[string]interface{}{
		"prettyA":   prettyA,
		"prettyB":   prettyB,
		"asciiDiff": asciiStr,
		"patch":     patchStr,
		"diffList":  dList,
	}
	if errA != nil { resp["errorA"] = errA }
	if errB != nil { resp["errorB"] = errB }

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func main() {
	fs := http.FileServer(http.Dir("./frontend"))
	http.Handle("/", fs)
	http.HandleFunc("/compare", compareHandler)

	port := "8080"
	fmt.Println("listening on http://localhost:" + port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
