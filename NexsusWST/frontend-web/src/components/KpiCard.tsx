"use client";
import {useEffect,useState} from "react";
import type { ReactNode } from "react";
export function KpiCard({ label, count, tone, icon }: {
    label: string;
    count: number | undefined;
    tone: string;
    icon: ReactNode;
}) {
    const [display,setDisplay]=useState(count);
    useEffect(()=>{if(count===undefined)return;if(display===undefined||matchMedia("(prefers-reduced-motion: reduce)").matches){setDisplay(count);return}const start=performance.now(),from=display;let frame=0;const tick=(now:number)=>{const progress=Math.min(1,(now-start)/300);setDisplay(Math.round(from+(count-from)*progress));if(progress<1)frame=requestAnimationFrame(tick)};frame=requestAnimationFrame(tick);return()=>cancelAnimationFrame(frame);
    // eslint-disable-next-line react-hooks/exhaustive-deps
    },[count]);
    return <article className={`kpi tone-${tone}`}>
    <h2>{label}</h2>
    <div>
    <strong>{display ?? "—"}</strong>{icon}</div>
    </article>;
}
