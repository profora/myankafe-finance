type Props={
  loading:boolean;
  empty:boolean;
  columns:number;
  emptyText:string;
  rows?:number;
};

export default function TableStateRows({loading,empty,columns,emptyText,rows=4}:Props){
  if(loading){
    return <>
      {Array.from({length:rows}).map((_,row)=>(
        <tr key={`loading-${row}`} aria-hidden="true">
          {Array.from({length:columns}).map((__,column)=>(
            <td key={column}><span className={column===0?"skeleton skeleton-wide":column%3===0?"skeleton skeleton-short":"skeleton skeleton-line"}/></td>
          ))}
        </tr>
      ))}
    </>;
  }
  if(empty){
    return <tr><td className="table-empty-cell" colSpan={columns}>{emptyText}</td></tr>;
  }
  return null;
}
