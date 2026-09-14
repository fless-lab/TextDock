<script setup>
import { withBase } from 'vitepress';
import release from '../../content/release.json';

const features = [
  ['Inspect messages', 'Search inboxes, inspect encoding and segments, copy OTPs, and export test messages.', '/guide/workflow'],
  ['Use your phone', 'Pair a read-only phone inbox by QR code. Messages arrive live over your local network.', '/guide/connect'],
  ['Test delivery behavior', 'Simulate delays, failures and incoming SMS. Inspect signed callbacks, retries and replays.', '/guide/simulation'],
  ['Connect your application', 'Use the HTTP API or the Node SDK with local, Twilio, Vonage and OVH drivers.', '/guide/providers'],
];
</script>

<template>
  <main class="home-page">
    <section class="intro" aria-labelledby="intro-heading">
      <div class="intro-copy">
        <p class="release-line"><span class="release-dot" /> Open source · {{ release.tag }}</p>
        <h1 id="intro-heading">Your local<br />SMS inbox.</h1>
        <p class="intro-description">Capture messages, inspect verification codes and test delivery flows. One binary, one local database, no provider account needed.</p>
        <div class="home-actions">
          <a class="home-primary" :href="withBase('/guide/getting-started.html')">Get started <span aria-hidden="true">→</span></a>
          <a class="home-secondary" :href="withBase('/downloads.html')">Download {{ release.tag }}</a>
        </div>
        <p class="runtime-note">Go + SQLite · Built-in web UI · Port 18257</p>
      </div>
      <div class="quick-example" aria-label="Local capture example">
        <div class="example-heading"><span>Terminal</span><span>Local capture</span></div>
        <pre><code><span class="command-comment"># Start TextDock</span>
./textdock

<span class="command-comment"># Send a test message</span>
curl localhost:18257/api/v1/messages \
  -H 'Content-Type: application/json' \
  -d '{
    "to": "+12025550123",
    "from": "Acme",
    "body": "Your code is 482193"
  }'</code></pre>
        <div class="example-result"><span>Result</span><code>captured</code><span>No carrier SMS sent</span></div>
      </div>
    </section>

    <section class="capabilities" aria-labelledby="capabilities-heading">
      <div class="section-intro"><h2 id="capabilities-heading">Built for the development loop</h2><p>From the first test message to automated end-to-end checks.</p></div>
      <div class="feature-grid"><article v-for="[title, text, link] in features" :key="title"><h3>{{ title }}</h3><p>{{ text }}</p><a :href="withBase(link + '.html')">Read the guide <span aria-hidden="true">→</span></a></article></div>
    </section>

    <section class="project-state" aria-labelledby="state-heading">
      <div><h2 id="state-heading">A working local tool.<br />A larger roadmap.</h2><p>The local inbox, phone pairing, simulation and provider SDK are available today. Integrated SMS relays, device gateways and managed hosting are the next milestones.</p><a :href="withBase('/project/status.html')">See what is ready and what comes next <span aria-hidden="true">→</span></a></div>
      <dl><div><dt>Run it</dt><dd>Standalone binary or Docker</dd></div><div><dt>Integrate it</dt><dd>HTTP API or Node SDK</dd></div><div><dt>Deploy it</dt><dd>Locally today; hosted service planned</dd></div><div><dt>Contribute</dt><dd>MIT license, public source and release checks</dd></div></dl>
    </section>

    <section class="home-resources" aria-label="Project resources"><a :href="withBase('/reference/api.html')">API reference</a><a :href="withBase('/project/changelog.html')">Changelog</a><a :href="withBase('/project/contributing.html')">Contribute</a><a href="https://github.com/fless-lab/TextDock/issues">Report an issue</a><a :href="withBase('/fr/projet.html')" lang="fr">Présentation en français</a></section>
  </main>
</template>
