"use client";

import { useEffect, useMemo, useState } from "react";
import { attachmentContentUrl, previewKind, type TransactionAttachment } from "@/lib/attachments";

type Props={
  entityID:string;
  transactionID:string;
  attachments:TransactionAttachment[];
  index:number|null;
  onClose:()=>void;
  onIndexChange:(index:number)=>void;
};

export default function AttachmentViewer({entityID,transactionID,attachments,index,onClose,onIndexChange}:Props){
  const [zoom,setZoom]=useState(1);
  const current=index==null?null:attachments[index]??null;
  const kind=current?previewKind(current.mime_type):"file";
  const src=useMemo(()=>current?attachmentContentUrl(entityID,transactionID,current.id):"",[entityID,transactionID,current?.id]);

  useEffect(()=>{setZoom(1)},[current?.id]);
  useEffect(()=>{
    if(index==null)return;
    const onKey=(e:KeyboardEvent)=>{
      if(e.key==="Escape")onClose();
      if(e.key==="ArrowLeft"&&index>0)onIndexChange(index-1);
      if(e.key==="ArrowRight"&&index<attachments.length-1)onIndexChange(index+1);
      if((e.key==="+"||e.key==="=")&&kind==="image")setZoom(z=>Math.min(5,z+.25));
      if(e.key==="-"&&kind==="image")setZoom(z=>Math.max(.5,z-.25));
      if(e.key==="0"&&kind==="image")setZoom(1);
    };
    window.addEventListener("keydown",onKey);
    return()=>window.removeEventListener("keydown",onKey);
  },[index,attachments.length,kind,onClose,onIndexChange]);

  if(index==null||!current)return null;

  return <div className="attachment-viewer" role="dialog" aria-modal="true" aria-label={current.original_filename} onClick={onClose}>
    <div className="attachment-viewer-toolbar" onClick={e=>e.stopPropagation()}>
      <div className="attachment-viewer-name">{current.original_filename}</div>
      <div className="actions">
        {kind==="image"&&<>
          <button className="viewer-button" onClick={()=>setZoom(z=>Math.max(.5,z-.25))}>−</button>
          <button className="viewer-button" onClick={()=>setZoom(1)}>{Math.round(zoom*100)}%</button>
          <button className="viewer-button" onClick={()=>setZoom(z=>Math.min(5,z+.25))}>+</button>
        </>}
        <a className="button viewer-button" href={src} target="_blank" rel="noreferrer">Open</a>
        <button className="viewer-button" onClick={onClose}>Close</button>
      </div>
    </div>

    <button className="attachment-nav attachment-prev" disabled={index===0} onClick={e=>{e.stopPropagation();onIndexChange(index-1)}} aria-label="Previous attachment">‹</button>
    <div className="attachment-viewer-stage" onClick={e=>e.stopPropagation()} onWheel={e=>{
      if(kind!=="image")return;
      e.preventDefault();
      setZoom(z=>Math.max(.5,Math.min(5,z+(e.deltaY<0?.15:-.15))));
    }}>
      {kind==="image"&&<img src={src} alt={current.original_filename} draggable={false} style={{transform:`scale(${zoom})`}}/>}
      {kind==="pdf"&&<iframe src={src} title={current.original_filename}/>}
      {kind==="file"&&<div className="attachment-file-fallback"><strong>{current.original_filename}</strong><p>This file type is not previewed in-browser.</p><a className="button" href={src} target="_blank" rel="noreferrer">Open file</a></div>}
    </div>
    <button className="attachment-nav attachment-next" disabled={index===attachments.length-1} onClick={e=>{e.stopPropagation();onIndexChange(index+1)}} aria-label="Next attachment">›</button>
    <div className="attachment-viewer-count">{index+1} / {attachments.length}</div>
  </div>;
}
