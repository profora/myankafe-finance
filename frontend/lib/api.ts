export const apiBase = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api/v1";

export class ApiError extends Error {
  status: number;
  constructor(status:number,message:string){
    super(message);
    this.status=status;
  }
}

function requestHeaders(init:RequestInit){
  const headers=new Headers(init.headers);
  if(init.body && !(init.body instanceof FormData) && !headers.has("Content-Type")){
    headers.set("Content-Type","application/json");
  }
  return headers;
}

function handleUnauthorized(path:string,status:number){
  if(status!==401||path==="/auth/login"||typeof window==="undefined")return;
  const next=encodeURIComponent(window.location.pathname+window.location.search);
  window.location.assign(`/login?expired=1&next=${next}`);
}

export async function api<T>(path:string,init:RequestInit={}):Promise<T>{
  const res=await fetch(`${apiBase}${path}`,{
    ...init,
    headers:requestHeaders(init),
    credentials:"include",
    cache:"no-store",
  });
  handleUnauthorized(path,res.status);
  const body=await res.json().catch(()=>({}));
  if(!res.ok)throw new ApiError(res.status,body.error??`HTTP ${res.status}`);
  return body as T;
}

export async function apiBlob(path:string):Promise<Blob>{
  const res=await fetch(`${apiBase}${path}`,{
    credentials:"include",
    cache:"no-store",
  });
  handleUnauthorized(path,res.status);
  if(!res.ok){
    const body=await res.json().catch(()=>({}));
    throw new ApiError(res.status,body.error??`HTTP ${res.status}`);
  }
  return res.blob();
}
