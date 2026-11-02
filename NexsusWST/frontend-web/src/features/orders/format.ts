import type {Order} from "@/services/orders";
export function channelLabel(channel?:string){if(!channel)return "—";return ({mercadolibre:"Mercado Libre",shopify:"Shopify"} as Record<string,string>)[channel]??channel.charAt(0).toUpperCase()+channel.slice(1)}
const capitalize=(value:string)=>value.charAt(0).toUpperCase()+value.slice(1);
export const DISPATCH_LABELS:Record<string,string>={domicilio:"Despacho a domicilio",retiro_oficina:"Retiro en oficina del courier",express:"Express (mismo día)"};
export function dispatchLabel(dispatchType?:string){if(!dispatchType)return "—";return DISPATCH_LABELS[dispatchType]??capitalize(dispatchType.replaceAll("_"," "))}
export function warehouseLabel(warehouse?:string){return warehouse?capitalize(warehouse):"—"}
export const ORIGIN_OPTIONS=[{value:"manual",label:"Orden manual"},{value:"bulk",label:"Carga masiva"},{value:"mercadolibre",label:"Mercado Libre"},{value:"shopify",label:"Shopify"}];
export const SITUATION_LABELS:Record<string,string>={en_cola:"En cola",invas_agotado:"Invas agotó reintentos",fuera_corte:"Fuera de corte",sin_regla:"Sin regla que calce"};
export function originLabel(order:Pick<Order,"source"|"channel">){return order.source==="manual"?"Orden manual":order.source==="bulk"?"Carga masiva":channelLabel(order.channel)}
