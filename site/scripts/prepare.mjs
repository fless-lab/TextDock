import { cp, mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../../', import.meta.url));
const site = path.join(root, 'site');
const content = path.join(site, 'content');
const version = (await readFile(path.join(root, 'VERSION'), 'utf8')).trim();
const repo = 'https://github.com/fless-lab/TextDock';
const release = { version, tag: `v${version}`, url: `${repo}/releases/tag/v${version}`, assets: `${repo}/releases/download/v${version}` };
const routes = new Map(Object.entries({
  'README.md': 'guide/getting-started.md',
  'docs/TEST-NUMBERS.md': 'guide/test-numbers.md',
  'docs/DEVELOPER-WORKFLOW.md': 'guide/workflow.md',
  'docs/CONNECT.md': 'guide/connect.md',
  'docs/SIMULATION.md': 'guide/simulation.md',
  'docs/PROVIDERS.md': 'guide/providers.md',
  'docs/RELAY.md': 'guide/relay.md',
  'docs/DEVICE-LAB.md': 'guide/device-lab.md',
  'docs/NOTIFICATIONS.md': 'guide/notifications.md',
  'docs/NATIVE-SAMPLES.md': 'guide/native-samples.md',
  'docs/API-KEYS.md': 'guide/api-keys.md',
  'android/README.md': 'reference/android-gateway.md',
  'docs/MOBILE-AND-OTP.md': 'guide/mobile-and-otp.md',
  'docs/API.md': 'reference/api.md',
  'docs/ARCHITECTURE.md': 'reference/architecture.md',
  'packages/sdk/README.md': 'reference/sdk.md',
  'SECURITY.md': 'project/security.md',
  'docs/ROADMAP.md': 'project/roadmap.md',
  'CHANGELOG.md': 'project/changelog.md',
  'CONTRIBUTING.md': 'project/contributing.md',
  'docs/RELEASING.md': 'project/releases.md',
  'docs/WEBSITE.md': 'project/website.md',
  'docs/UI.md': 'project/ui.md',
  'docs/PROJET.fr.md': 'fr/projet.md',
  'LICENSE': 'project/license.md',
}));

for (const filename of await readdir(path.join(root, 'docs'))) {
  if (filename.startsWith('VALIDATION-') && filename.endsWith('.md')) routes.set(`docs/${filename}`, `validation/${filename.slice('VALIDATION-'.length)}`);
  else if (filename.endsWith('.md') && !routes.has(`docs/${filename}`)) throw new Error(`Add docs/${filename} to the website route map`);
}

function expand(text) {
  return text.replaceAll('{{VERSION}}', version).replaceAll('{{RELEASE_URL}}', release.url).replaceAll('{{ASSET_URL}}', release.assets);
}
function rewriteLinks(text, source) {
  return text.replace(/(!?\[[^\]]*\]\()([^\s)]+)([^)]*\))/g, (whole, prefix, target, suffix) => {
    if (/^(?:[a-z][a-z\d+.-]*:|\/|#)/i.test(target)) return whole;
    const [relative, fragment] = target.split('#');
    const file = path.posix.normalize(path.posix.join(path.posix.dirname(source), relative));
    const anchor = fragment ? `#${fragment}` : '';
    const route = routes.get(file);
    if (route) return `${prefix}/${route.replace(/\.md$/, '')}${anchor}${suffix}`;
    if (file.startsWith('api/') && file.endsWith('.yaml')) return `${prefix}/${file}${anchor}${suffix}`;
    if (file.endsWith('.md')) throw new Error(`Unmapped Markdown link in ${source}: ${target}`);
    return `${prefix}${repo}/${file.endsWith('/') ? 'tree' : 'blob'}/${release.tag}/${file}${anchor}${suffix}`;
  });
}
async function output(relative, text) {
  const destination = path.join(content, relative);
  await mkdir(path.dirname(destination), { recursive: true });
  await writeFile(destination, text);
}

// content/ contains generated material only; canonical Markdown stays in docs/.
await rm(content, { recursive: true, force: true });
await mkdir(content, { recursive: true });
for (const [source, route] of routes) {
  let text = await readFile(path.join(root, source), 'utf8');
  if (source === 'LICENSE') text = `# License\n\n${text}`;
  text = rewriteLinks(text, source);
  await output(route, `<!-- Generated from ${source}; edit the canonical source. -->\n\n${text}`);
}
async function copyPages(directory, relative = '') {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const target = path.posix.join(relative, entry.name);
    if (entry.isDirectory()) await copyPages(path.join(directory, entry.name), target);
    else if (entry.name.endsWith('.md')) await output(target, expand(await readFile(path.join(directory, entry.name), 'utf8')));
  }
}
await copyPages(path.join(site, 'pages'));
await output('release.json', JSON.stringify(release));
await mkdir(path.join(content, 'public', 'api'), { recursive: true });
for (const file of await readdir(path.join(root, 'api'))) {
  if (file.endsWith('.yaml')) await cp(path.join(root, 'api', file), path.join(content, 'public', 'api', file));
}
await cp(path.join(root, 'web/public/favicon.svg'), path.join(content, 'public/favicon.svg'));
await writeFile(path.join(content, 'public/.nojekyll'), '');
await writeFile(path.join(content, 'public/robots.txt'), 'User-agent: *\nAllow: /TextDock/\nSitemap: https://fless-lab.github.io/TextDock/sitemap.xml\n');
console.log(`Prepared ${routes.size} canonical documents and website pages for ${release.tag}.`);
