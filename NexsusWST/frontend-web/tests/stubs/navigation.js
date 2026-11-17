const nav=()=>globalThis.__nav??(globalThis.__nav={params:new URLSearchParams(),calls:[]});
export const useRouter=()=>({push:u=>nav().calls.push(['push',u]),replace:u=>nav().calls.push(['replace',u]),back:()=>{},refresh:()=>{}});
export const usePathname=()=>nav().pathname??'/ordenes/todas/listado';
export const useSearchParams=()=>nav().params;
export const redirect=u=>{nav().calls.push(['redirect',u])};
