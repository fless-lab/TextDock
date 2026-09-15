// Deterministic PNG assets for browser install prompts and notification badges.
// Uses only Node standard modules; the application's runtime needs no renderer.
import { writeFile, mkdir } from 'node:fs/promises';
import { deflateSync } from 'node:zlib';

function crc32(bytes) {
  let crc = 0xffffffff;
  for (const byte of bytes) { crc ^= byte; for (let bit = 0; bit < 8; bit++) crc = (crc >>> 1) ^ (0xedb88320 & -(crc & 1)); }
  return (crc ^ 0xffffffff) >>> 0;
}
function chunk(type, data) {
  const name = Buffer.from(type); const length = Buffer.alloc(4); length.writeUInt32BE(data.length);
  const crc = Buffer.alloc(4); crc.writeUInt32BE(crc32(Buffer.concat([name, data])));
  return Buffer.concat([length, name, data, crc]);
}
const lines = [[17,18,47,18],[47,18,47,41],[47,41,30,41],[30,41,21,48],[21,48,21,41],[21,41,17,41],[17,41,17,18],[25,27,39,27],[25,33,34,33]];
function distance(x, y, [a,b,c,d]) { const t = Math.max(0, Math.min(1, ((x-a)*(c-a)+(y-b)*(d-b))/((c-a)**2+(d-b)**2))); return Math.hypot(x-a-t*(c-a), y-b-t*(d-b)); }
function png(size, badge) {
  const data = Buffer.alloc(size * (size * 4 + 1));
  for (let y = 0; y < size; y++) for (let x = 0; x < size; x++) {
    const stroke = lines.some(line => distance((x+.5)*64/size, (y+.5)*64/size, line) < 1.9);
    const offset = y*(size*4+1)+1+x*4;
    const color = stroke ? [255,255,255,255] : badge ? [0,0,0,0] : [234,102,59,255];
    for (let channel = 0; channel < 4; channel++) data[offset+channel] = color[channel];
  }
  const header = Buffer.alloc(13); header.writeUInt32BE(size, 0); header.writeUInt32BE(size, 4); header[8] = 8; header[9] = 6;
  return Buffer.concat([Buffer.from([137,80,78,71,13,10,26,10]), chunk('IHDR', header), chunk('IDAT', deflateSync(data)), chunk('IEND', Buffer.alloc(0))]);
}
const directory = new URL('../public/', import.meta.url);
await mkdir(directory, { recursive: true });
for (const size of [192,512]) await writeFile(new URL(`phone-icon-${size}.png`, directory), png(size, false));
await writeFile(new URL('phone-badge-96.png', directory), png(96, true));
