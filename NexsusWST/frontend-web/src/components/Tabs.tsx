import Link from "next/link";
export function Tabs({ items, label = "Vistas de órdenes" }: {
    items: {
        href: string;
        label: string;
        active: boolean;
    }[];
    label?: string;
}) {
    return <nav className="view-tabs" aria-label={label}>{items.map(i => <Link key={i.href} href={i.href} aria-current={i.active ? "page" : undefined}>{i.label}</Link>)}</nav>;
}
