"use client";
import {useEffect,useState} from "react";
import type { ReactNode } from "react";
export function KpiCard({ label, count, tone, icon, active = false, onClick }: {
    label: string;
    count: number | undefined;
    tone: string;
    icon: ReactNode;
    active?: boolean;
    onClick?: () => void; // sin onClick, la tarjeta es solo informativa
}) {
    const [display,setDisplay]=useState(count);
    useEffect(()=>{if(count===undefined)return;if(display===undefined||matchMedia("(prefers-reduced-motion: reduce)").matches){setDisplay(count);return}const start=performance.now(),from=display;let frame=0;const tick=(now:number)=>{const progress=Math.min(1,(now-start)/300);setDisplay(Math.round(from+(count-from)*progress));if(progress<1)frame=requestAnimationFrame(tick)};frame=requestAnimationFrame(tick);return()=>cancelAnimationFrame(frame);
    // eslint-disable-next-line react-hooks/exhaustive-deps
    },[count]);
    const content = <>
    <span className="kpi-label">{label}</span>
    <span className="kpi-row">
    <strong>{display ?? "—"}</strong>{icon}</span>
    </>;
    if (!onClick) return <article className={`kpi kpi-info tone-${tone}`}>{content}</article>;
    return <button type="button" className={`kpi kpi-button tone-${tone} ${active?"is-active":""}`} aria-pressed={active} onClick={onClick}>{content}</button>;
}
