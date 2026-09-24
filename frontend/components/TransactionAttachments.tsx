"use client";

import { ChangeEvent, useEffect, useRef, useState } from "react";
import AttachmentViewer from "@/components/AttachmentViewer";
import {
  attachmentContentUrl,
  formatAttachmentSize,
  listTransactionAttachments,
  previewKind,
  uploadTransactionAttachments,
  type TransactionAttachment,
} from "@/lib/attachments";

type Props={entityID:string;transactionID:string;canUpload?:boolean};

export default function TransactionAttachments({entityID,transactionID,canUpload=true}:Props){
  const [items,setItems]=useState<TransactionAttachment[]>([]);
  const [viewerIndex,setViewerIndex]=useState<number|null>(null);
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState("");
  const inputRef=useRef<HTMLInputElement>(null);

  async function load(){
    try{const x=await listTransactionAttachments(entityID,transactionID);setItems(x.items)}
    catch(e){setError(e instanceof Error?e.message:String(e))}
  }
  useEffect(()=>{load()},[entityID,transactionID]);

  async function selectFiles(e:ChangeEvent<HTMLInputElement>){
    const files=Array.from(e.target.files??[]);
    e.target.value="";
    if(!files.length)return;
    setBusy(true);setError("");
    try{
      await uploadTransactionAttachments(entityID,transactionID,files);
      await load();
    }catch(err){setError(err instanceof Error?err.message:String(err))}
    finally{setBusy(false)}
  }

  return <>
    <div className="page-head" style={{marginTop:28}}>
      <div><h1 style={{fontSize:20}}>Attachments</h1><p>Receipts, invoices, images, PDFs and supporting files.</p></div>
      {canUpload&&<>
        <input ref={inputRef} hidden type="file" multiple accept="image/jpeg,image/png,image/webp,image/gif,application/pdf,text/plain,text/csv,.docx,.xlsx" onChange={selectFiles}/>
        <button disabled={busy} onClick={()=>inputRef.current?.click()}>{busy?"Uploading…":"Attach files"}</button>
      </>}
    </div>
    {error&&<div className="alert error">{error}</div>}
    {items.length?<div className="attachment-grid">{items.map((item,index)=>{
      const kind=previewKind(item.mime_type);
      const src=attachmentContentUrl(entityID,transactionID,item.id);
      return <button key={item.id} className="attachment-card" onClick={()=>setViewerIndex(index)}>
        <div className="attachment-thumb">
          {kind==="image"?<img src={src} alt=""/>:<span>{kind==="pdf"?"PDF":"FILE"}</span>}
        </div>
        <div className="attachment-card-copy">
          <strong title={item.original_filename}>{item.original_filename}</strong>
          <span>{formatAttachmentSize(item.size_bytes)}</span>
        </div>
      </button>;
    })}</div>:<div className="empty attachment-empty">No attachments yet.</div>}
    <AttachmentViewer entityID={entityID} transactionID={transactionID} attachments={items} index={viewerIndex} onClose={()=>setViewerIndex(null)} onIndexChange={setViewerIndex}/>
  </>;
}
