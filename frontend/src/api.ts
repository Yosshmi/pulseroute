export class APIError extends Error { constructor(public status:number,message:string){super(message)} }
export async function api<T>(path:string,init:RequestInit={}):Promise<T>{const response=await fetch(path,{...init,credentials:'same-origin',headers:{'Content-Type':'application/json',...init.headers}});if(!response.ok){const body=await response.json().catch(()=>null);throw new APIError(response.status,body?.error?.message??`Request failed (${response.status})`)};return response.status===204?undefined as T:response.json()}
export function post<T>(path:string,body:unknown){return api<T>(path,{method:'POST',body:JSON.stringify(body)})}
export type Project={id:string;name:string;created_at:string};
export type Endpoint={id:string;name:string;url:string;active:boolean;max_attempts:number;base_delay_seconds:number;rate_per_second:number};
export type Event={id:string;event_id:string;type:string;created_at:string;payload?:unknown;deliveries?:Delivery[]};
export type Attempt={id:string;attempt:number;generation:number;status_code:number;duration_ms:number;error:string;created_at:string};
export type Delivery={id:string;event_id:string;external_event_id?:string;type?:string;endpoint_id:string;endpoint_name:string;status:string;attempt_count:number;generation:number;created_at:string;next_attempt_at:string;reason?:string;attempts?:Attempt[]};
export type Key={id:string;name:string;prefix:string;revoked_at:string|null};
export type Page<T>={items:T[];limit:number;offset:number};
export const timestamp=(value:string)=>new Date(value).toLocaleString();
