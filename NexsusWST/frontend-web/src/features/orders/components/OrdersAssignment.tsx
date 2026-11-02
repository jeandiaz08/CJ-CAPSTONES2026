"use client";
import {useEffect} from "react";
import {useRouter} from "next/navigation";
import {useAuth} from "@/state/auth";
import {OrdersListing} from "./OrdersListing";
export function OrdersAssignment(){const {session}=useAuth();const router=useRouter();const allowed=session?.user.role==="admin"||session?.user.role==="jefe_operaciones";useEffect(()=>{if(session&&!allowed)router.replace("/ordenes/todas/listado")},[session,allowed,router]);return allowed?<OrdersListing assignment/>:null}
