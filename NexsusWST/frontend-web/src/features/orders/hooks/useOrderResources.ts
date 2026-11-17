"use client";
import { useState, useEffect, useRef } from "react";
export function useOrderResource<T>(load: () => Promise<T>, key: string, identity: string = key) {
    const [state, setState] = useState<{
        data?: T;
        error?: string;
        loading: boolean;
    }>({ loading: true });
    const lastIdentity = useRef(identity);
    useEffect(() => {
        let current = true;
        const same = lastIdentity.current === identity;
        lastIdentity.current = identity;
        setState(previous => same && previous.data !== undefined ? { ...previous, loading: false } : { loading: true });
        load().then(data => { if (current)
            setState({ data, loading: false }); }).catch(e => { if (current)
            setState(previous => ({ ...previous, error: e.message, loading: false })); });
        return () => { current = false; };
        // key identifies the request including company, selection and refresh version.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [key]);
    return state;
}
