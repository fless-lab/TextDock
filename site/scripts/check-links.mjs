import assert from 'node:assert/strict';
import { readFile, readdir, stat } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const dist = fileURLToPath(new URL('../.vitepress/dist/', import.meta.url));
const origin = 'https://fless-lab.github.io';
const base = '/TextDock/';
const html = new Map();
async function collect(directory) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const filename = path.join(directory, entry.name);
    if (entry.isDirectory()) await collect(filename);
    else if (entry.name.endsWith('.html')) html.set(path.relative(dist, filename).split(path.sep).join('/'), await readFile(filename, 'utf8'));
  }
}
await collect(dist);
assert.ok(html.size >= 25, 'The full documentation must be included');
const errors = [];
let links = 0;
for (const [filename, source] of html) {
  assert.ok(!source.includes('{{VERSION}}') && !source.includes('{{ASSET_URL}}'), `Unexpanded metadata in ${filename}`);
  for (const match of source.matchAll(/\b(?:href|src)="([^"\s]+)"/g)) {
    const target = new URL(match[1].replaceAll('&amp;', '&'), `${origin}${base}${filename}`);
    if (target.origin !== origin) continue;
    links++;
    if (!target.pathname.startsWith(base)) { errors.push(`${filename}: link escaped project base: ${target.pathname}`); continue; }
    let relative = decodeURIComponent(target.pathname.slice(base.length));
    if (!relative || relative.endsWith('/')) relative += 'index.html';
    const destination = path.join(dist, relative);
    try { if (!(await stat(destination)).isFile()) throw new Error('not a file'); }
    catch { errors.push(`${filename}: missing ${relative}`); continue; }
    if (target.hash && relative.endsWith('.html')) {
      const id = decodeURIComponent(target.hash.slice(1));
      if (!html.get(relative)?.includes(`id="${id}"`)) errors.push(`${filename}: missing anchor ${relative}#${id}`);
    }
  }
}
assert.equal(errors.length, 0, errors.join('\n'));
const sitemap = await readFile(path.join(dist, 'sitemap.xml'), 'utf8');
assert.ok(sitemap.includes(`${origin}${base}`), 'Sitemap needs the project base URL');
assert.ok(!sitemap.includes('/TextDock/TextDock/'), 'Sitemap must not duplicate the project base');
console.log(`Validated ${links} local links/assets across ${html.size} pages, including anchors and project-base paths.`);
