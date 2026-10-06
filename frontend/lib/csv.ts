export type CsvCell = string | number | boolean | null | undefined;

function csvCell(value:CsvCell){
  const text=value==null?"":String(value);
  return `"${text.replaceAll('"','""')}"`;
}

export function downloadCsv(filename:string,headers:string[],rows:CsvCell[][]){
  const content=[
    headers.map(csvCell).join(","),
    ...rows.map(row=>row.map(csvCell).join(",")),
  ].join("\r\n");
  const blob=new Blob(["\ufeff",content],{type:"text/csv;charset=utf-8"});
  const url=URL.createObjectURL(blob);
  const a=document.createElement("a");
  a.href=url;
  a.download=filename.endsWith(".csv")?filename:`${filename}.csv`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(()=>URL.revokeObjectURL(url),1000);
}

export function safeCsvFilename(value:string){
  return value.trim().replace(/[^a-zA-Z0-9._-]+/g,"-").replace(/^-+|-+$/g,"").toLowerCase()||"export";
}
