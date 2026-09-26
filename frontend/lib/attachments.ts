import { api, apiBase } from "@/lib/api";

export type TransactionAttachment={
  id:string;
  original_filename:string;
  mime_type:string;
  size_bytes:number;
  display_order:number;
  created_at:string;
};

export function attachmentContentUrl(entityID:string,transactionID:string,attachmentID:string){
  return `${apiBase}/entities/${encodeURIComponent(entityID)}/transactions/${encodeURIComponent(transactionID)}/attachments/${encodeURIComponent(attachmentID)}/content`;
}

export async function listTransactionAttachments(entityID:string,transactionID:string){
  return api<{items:TransactionAttachment[]}>(`/entities/${entityID}/transactions/${transactionID}/attachments`);
}

export async function uploadTransactionAttachments(entityID:string,transactionID:string,files:File[]){
  const form=new FormData();
  files.forEach(file=>form.append("files",file,file.name));
  return api<{items:TransactionAttachment[]}>(`/entities/${entityID}/transactions/${transactionID}/attachments`,{
    method:"POST",
    body:form,
  });
}

export function formatAttachmentSize(bytes:number){
  if(bytes<1024)return `${bytes} B`;
  if(bytes<1024*1024)return `${(bytes/1024).toFixed(1)} KB`;
  return `${(bytes/(1024*1024)).toFixed(1)} MB`;
}

export function previewKind(mime:string){
  if(mime.startsWith("image/"))return "image";
  if(mime==="application/pdf")return "pdf";
  if(mime==="text/plain"||mime==="text/csv")return "text";
  return "file";
}


export async function deleteTransactionAttachment(entityID:string,transactionID:string,attachmentID:string){
  await api(`/entities/${entityID}/transactions/${transactionID}/attachments/${attachmentID}`,{method:"DELETE"});
}

export async function reorderTransactionAttachments(entityID:string,transactionID:string,attachmentIDs:string[]){
  return api<{items:TransactionAttachment[]}>(`/entities/${entityID}/transactions/${transactionID}/attachments/reorder`,{
    method:"PUT",
    body:JSON.stringify({attachment_ids:attachmentIDs}),
  });
}
