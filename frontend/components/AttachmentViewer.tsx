"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { attachmentContentUrl, previewKind, type TransactionAttachment } from "@/lib/attachments";

type Props={
  entityID:string;
  transactionID:string;
  attachments:TransactionAttachment[];
  index:number|null;
  onClose:()=>void;
  onIndexChange:(index:number)=>void;
};

type Pan={x:number;y:number};

export default function AttachmentViewer({entityID,transactionID,attachments,index,onClose,onIndexChange}:Props){
  const [zoom,setZoom]=useState(1);
  const [pan,setPan]=useState<Pan>({x:0,y:0});
  const [dragging,setDragging]=useState(false);
  const dialogRef=useRef<HTMLDivElement>(null);
  const dragRef=useRef<{pointerId:number;startX:number;startY:number;panX:number;panY:number}|null>(null);
  const current=index==null?null:attachments[index]??null;
  const kind=current?previewKind(current.mime_type):"file";
  const src=useMemo(()=>current?attachmentContentUrl(entityID,transactionID,current.id):"",[entityID,transactionID,current?.id]);

  function resetView(){
    setZoom(1);
    setPan({x:0,y:0});
    setDragging(false);
    dragRef.current=null;
  }

  function changeZoom(next:(current:number)=>number){
    setZoom(current=>{
      const value=Math.max(.5,Math.min(5,next(current)));
      if(value<=1)setPan({x:0,y:0});
      return value;
    });
  }

  useEffect(()=>{
    resetView();
    if(current)requestAnimationFrame(()=>dialogRef.current?.focus());
  },[current?.id]);
  useEffect(()=>{
    if(index==null)return;
    const onKey=(e:KeyboardEvent)=>{
      if(e.key==="Escape")onClose();
      if(e.key==="ArrowLeft"&&index>0)onIndexChange(index-1);
      if(e.key==="ArrowRight"&&index<attachments.length-1)onIndexChange(index+1);
      if((e.key==="+"||e.key==="=")&&kind==="image")changeZoom(z=>z+.25);
      if(e.key==="-"&&kind==="image")changeZoom(z=>z-.25);
      if(e.key==="0"&&kind==="image")resetView();
    };
    window.addEventListener("keydown",onKey);
    return()=>window.removeEventListener("keydown",onKey);
  },[index,attachments.length,kind,onClose,onIndexChange]);

  if(index==null||!current)return null;

  return <div ref={dialogRef} tabIndex={-1} className="attachment-viewer" role="dialog" aria-modal="true" aria-label={`Attachment preview: ${current.original_filename}`} onClick={onClose}>
    <div className="attachment-viewer-toolbar" onClick={e=>e.stopPropagation()}>
      <div className="attachment-viewer-name">{current.original_filename}</div>
      <div className="actions">
        {kind==="image"&&<>
          <button type="button" className="viewer-button" aria-label="Zoom out" onClick={()=>changeZoom(z=>z-.25)}>−</button>
          <button type="button" className="viewer-button" aria-label="Reset zoom" onClick={resetView}>{Math.round(zoom*100)}%</button>
          <button type="button" className="viewer-button" aria-label="Zoom in" onClick={()=>changeZoom(z=>z+.25)}>+</button>
        </>}
        <a className="button viewer-button" href={src} target="_blank" rel="noreferrer">Open</a>
        <button type="button" className="viewer-button" onClick={onClose}>Close</button>
      </div>
    </div>

    <button type="button" className="attachment-nav attachment-prev" disabled={index===0} onClick={e=>{e.stopPropagation();onIndexChange(index-1)}} aria-label="Previous attachment">‹</button>
    <div className="attachment-viewer-stage" onClick={e=>e.stopPropagation()} onWheel={e=>{
      if(kind!=="image")return;
      e.preventDefault();
      changeZoom(z=>z+(e.deltaY<0?.15:-.15));
    }}>
      {kind==="image"&&<img
        src={src}
        alt={current.original_filename}
        draggable={false}
        className={zoom>1?(dragging?"zoomed dragging":"zoomed"):""}
        style={{transform:`translate(${pan.x}px,${pan.y}px) scale(${zoom})`}}
        onDoubleClick={resetView}
        onPointerDown={e=>{
          if(zoom<=1)return;
          e.currentTarget.setPointerCapture(e.pointerId);
          dragRef.current={pointerId:e.pointerId,startX:e.clientX,startY:e.clientY,panX:pan.x,panY:pan.y};
          setDragging(true);
        }}
        onPointerMove={e=>{
          const drag=dragRef.current;
          if(!drag||drag.pointerId!==e.pointerId)return;
          setPan({x:drag.panX+(e.clientX-drag.startX),y:drag.panY+(e.clientY-drag.startY)});
        }}
        onPointerUp={e=>{
          if(dragRef.current?.pointerId===e.pointerId){
            dragRef.current=null;setDragging(false);
            e.currentTarget.releasePointerCapture(e.pointerId);
          }
        }}
        onPointerCancel={()=>{dragRef.current=null;setDragging(false)}}
      />}
      {(kind==="pdf"||kind==="text")&&<iframe src={src} title={current.original_filename}/>}
      {kind==="file"&&<div className="attachment-file-fallback"><strong>{current.original_filename}</strong><p>This file type is not previewed in-browser.</p><a className="button" href={src} target="_blank" rel="noreferrer">Open file</a></div>}
    </div>
    <button type="button" className="attachment-nav attachment-next" disabled={index===attachments.length-1} onClick={e=>{e.stopPropagation();onIndexChange(index+1)}} aria-label="Next attachment">›</button>
    <div className="attachment-viewer-count">{index+1} / {attachments.length}</div>
  </div>;
}
