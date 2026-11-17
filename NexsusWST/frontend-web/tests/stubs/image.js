import {createElement} from 'react';
export default function Image({src,alt,width,height,onError,className}){return createElement('img',{src,alt,width,height,onError,className})}
