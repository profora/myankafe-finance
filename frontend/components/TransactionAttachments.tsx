"use client";

import { ChangeEvent, useEffect, useRef, useState } from "react";
import AttachmentViewer from "@/components/AttachmentViewer";
import ConfirmDialog from "@/components/ConfirmDialog";
import {
  attachmentContentUrl,
  deleteTransactionAttachment,
  formatAttachmentSize,
  listTransactionAttachments,
  previewKind,
  reorderTransactionAttachments,
  uploadTransactionAttachments,
  type TransactionAttachment,
} from "@/lib/attachments";

type Props={entityID:string;transactionID:string;canUpload?:boolean;canManage?:boolean};

export default function TransactionAttachments({entityID,transactionID,canUpload=true,canManage=true}:Props){
  const [items,setItems]=useState<TransactionAttachment[]>([]);
  const [viewerIndex,setViewerIndex]=useState<number|null>(null);
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState("");
  const [deleteTarget,setDeleteTarget]=useState<TransactionAttachment|null>(null);
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

  async function move(index:number,direction:-1|1){
    const next=index+direction;
    if(next<0||next>=items.length)return;
    const reordered=[...items];
    [reordered[index],reordered[next]]=[reordered[next],reordered[index]];
    setBusy(true);setError("");
    try{
      const response=await reorderTransactionAttachments(entityID,transactionID,reordered.map(x=>x.id));
      setItems(response.items);
      if(viewerIndex===index)setViewerIndex(next);
      else if(viewerIndex===next)setViewerIndex(index);
    }catch(err){setError(err instanceof Error?err.message:String(err))}
    finally{setBusy(false)}
  }

  async function remove(){
    if(!deleteTarget)return;
    setBusy(true);setError("");
    try{
      await deleteTransactionAttachment(entityID,transactionID,deleteTarget.id);
      setDeleteTarget(null);setViewerIndex(null);
      await load();
    }catch(err){setError(err instanceof Error?err.message:String(err))}
    finally{setBusy(false)}
  }

  return <>
    <div className="page-head" style={{marginTop:28}}>
      <div><h1 style={{fontSize:20}}>Attachments</h1><p>Receipts, invoices, images, PDFs and supporting files.</p></div>
      {canUpload&&<>
        <input ref={inputRef} hidden type="file" multiple accept="image/jpeg,image/png,image/webp,image/gif,application/pdf,text/plain,text/csv,.docx,.xlsx" onChange={selectFiles}/>
        <button disabled={busy} onClick={()=>inputRef.current?.click()}>{busy?"Working…":"Attach files"}</button>
      </>}
    </div>
    {error&&<div className="alert error">{error}</div>}
    {items.length?<div className="attachment-grid">{items.map((item,index)=>{
      const kind=previewKind(item.mime_type);
      const src=attachmentContentUrl(entityID,transactionID,item.id);
      return <article key={item.id} className="attachment-card">
        <button className="attachment-preview-button" onClick={()=>setViewerIndex(index)}>
          <div className="attachment-thumb">
            {kind==="image"?<img src={src} alt=""/>:<span>{kind==="pdf"?"PDF":"FILE"}</span>}
          </div>
          <div className="attachment-card-copy">
            <strong title={item.original_filename}>{item.original_filename}</strong>
            <span>{formatAttachmentSize(item.size_bytes)}</span>
          </div>
        </button>
        {canManage&&<div className="attachment-card-actions">
          <button className="secondary compact" disabled={busy||index===0} onClick={()=>move(index,-1)} aria-label="Move attachment earlier">←</button>
          <button className="secondary compact" disabled={busy||index===items.length-1} onClick={()=>move(index,1)} aria-label="Move attachment later">→</button>
          <button className="danger compact" disabled={busy} onClick={()=>setDeleteTarget(item)}>Remove</button>
        </div>}
      </article>;
    })}</div>:<div className="empty attachment-empty">No attachments yet.</div>}
    <AttachmentViewer entityID={entityID} transactionID={transactionID} attachments={items} index={viewerIndex} onClose={()=>setViewerIndex(null)} onIndexChange={setViewerIndex}/>
    <ConfirmDialog
      open={Boolean(deleteTarget)}
      title="Remove attachment?"
      description="The attachment will disappear from the transaction. The removal itself remains in the audit history."
      confirmLabel="Remove attachment"
      danger
      busy={busy}
      onCancel={()=>{if(!busy)setDeleteTarget(null)}}
      onConfirm={remove}
    >
      {deleteTarget&&<p><strong>{deleteTarget.original_filename}</strong></p>}
    </ConfirmDialog>
  </>;
}
