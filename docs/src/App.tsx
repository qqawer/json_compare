import React, { useState, useEffect } from 'react'

export default function App(){
  const [a, setA] = useState('')
  const [b, setB] = useState('')
  const [ascii, setAscii] = useState(
    "-- The comparison results will be displayed here --",
  );
  const [diffList, setDiffList] = useState<any[]>([])
  const [errorA, setErrorA] = useState<string | null>(null)
  const [errorB, setErrorB] = useState<string | null>(null)

  useEffect(()=>{
    const id = setTimeout(()=>{
      if(a.trim() !== '' && b.trim() !== '') doCompare()
    }, 400)
    return ()=> clearTimeout(id)
  }, [a,b])

  async function doCompare(){
    // simple client parse
    try{ JSON.parse(a); setErrorA(null)}catch(e:any){ setErrorA(e.message); setAscii('JSON 解析错误'); return }
    try{ JSON.parse(b); setErrorB(null)}catch(e:any){ setErrorB(e.message); setAscii('JSON 解析错误'); return }
    setAscii('比较中...')
    const fd = new FormData()
    fd.append('fileA', new Blob([a], {type:'application/json'}), 'a.json')
    fd.append('fileB', new Blob([b], {type:'application/json'}), 'b.json')
    try{
      const res = await fetch('http://localhost:8080/compare', { method: 'POST', body: fd })
      const j = await res.json()
      if(j.errorA) setErrorA(j.errorA.message || j.errorA)
      if(j.errorB) setErrorB(j.errorB.message || j.errorB)
      if(j.prettyA) setA(j.prettyA)
      if(j.prettyB) setB(j.prettyB)
      setAscii(j.asciiDiff || '-- 无 ascii 差异 --')
      setDiffList(j.diffList || [])
    }catch(e:any){ setAscii('比较失败: '+ e.message) }
  }

  function download(filename: string, text: string){
    const a = document.createElement('a')
    a.href = URL.createObjectURL(new Blob([text], {type: 'application/json'}))
    a.download = filename
    document.body.appendChild(a); a.click(); a.remove()
  }

  return (
    <div className="p-6 font-sans">
      <h1 className="text-2xl mb-4">JSON Compare (React)</h1>
      <div className="grid grid-cols-2 gap-4">
        <div>
          <h3 className="mb-2">Pretty A</h3>
          <textarea className="w-full h-72 p-2 border rounded" value={a} onChange={e=>setA(e.target.value)} />
          {errorA && <div className="text-red-600 mt-2">解析错误: {errorA}</div>}
          <div className="flex gap-2 mt-2">
            <input type="file" onChange={async e=>{ const f = e.target.files?.[0]; if(f){ setA(await f.text()) } }} />
            <button className="px-3 py-1 bg-gray-200 rounded" onClick={()=>{ try{ setA(JSON.stringify(JSON.parse(a), null, 2)); }catch(e){ } }}>格式化</button>
            <button className="px-3 py-1 bg-gray-200 rounded" onClick={()=>download('prettyA.json', a)}>下载 A</button>
          </div>
        </div>
        <div>
          <h3 className="mb-2">Pretty B</h3>
          <textarea className="w-full h-72 p-2 border rounded" value={b} onChange={e=>setB(e.target.value)} />
          {errorB && <div className="text-red-600 mt-2">解析错误: {errorB}</div>}
          <div className="flex gap-2 mt-2">
            <input type="file" onChange={async e=>{ const f = e.target.files?.[0]; if(f){ setB(await f.text()) } }} />
            <button className="px-3 py-1 bg-gray-200 rounded" onClick={()=>{ try{ setB(JSON.stringify(JSON.parse(b), null, 2)); }catch(e){ } }}>格式化</button>
            <button className="px-3 py-1 bg-gray-200 rounded" onClick={()=>download('prettyB.json', b)}>下载 B</button>
          </div>
        </div>
      </div>

      <div className="mt-6">
        <h3 className="mb-2">Diff (ASCII)</h3>
        <pre className="p-3 bg-gray-100 rounded">{ascii}</pre>

        <h3 className="mt-4 mb-2">Structured differences</h3>
        <div>
          {diffList.length===0 && <div className="text-sm text-gray-600">暂无差异或尚未比较</div>}
          {diffList.length>0 && (
            <table className="w-full border-collapse">
              <thead><tr className="text-left"><th>Path</th><th>Type</th><th>A</th><th>B</th></tr></thead>
              <tbody>
                {diffList.map((d,i)=> (
                  <tr key={i} className={d.type==='added'? 'bg-green-50': d.type==='removed'? 'bg-red-50':'bg-yellow-50'}>
                    <td className="p-2">{d.path}</td>
                    <td className="p-2">{d.type}</td>
                    <td className="p-2">{JSON.stringify(d.a)}</td>
                    <td className="p-2">{JSON.stringify(d.b)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

      </div>
    </div>
  )
}
