"use client";
import {useState} from "react";
import Image from "next/image";
import {courierLogos} from "../courierLogos";

// Logo del courier, o un chip con su nombre si no hay logo o no carga.
export function CourierMark({code,name}:{code:string;name:string}){
  const [failed,setFailed]=useState(false);
  const logo=courierLogos[code];
  return logo&&!failed?<Image className="courier-logo" src={logo} alt={name} width={90} height={24} onError={()=>setFailed(true)}/>:<span className="courier-chip">{name}</span>;
}
