import { listProducts, type Product } from "./core";
const products = new Map<number, Promise<Product[]>>();
export function productsForCompany(companyId: number) {
    let request = products.get(companyId);
    if (!request) {
        request = listProducts("", companyId).then(r => r.data).catch(e => { products.delete(companyId); throw e; });
        products.set(companyId, request);
    }
    return request;
}
export function clearProductCache() { products.clear(); }
