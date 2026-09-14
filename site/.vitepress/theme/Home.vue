<script setup>
import { withBase } from 'vitepress';
import release from '../../content/release.json';
</script>

<template>
  <main class="project-home">
    <header class="project-heading">
      <h1>TextDock</h1>
      <p>SMS development server</p>
    </header>

    <div class="project-columns">
      <div class="project-content">
        <p>TextDock captures SMS sent by your application and displays them in a local web interface. Use it to inspect messages, retrieve verification codes and test delivery callbacks.</p>
        <p>It runs as a single binary with SQLite storage. The default capture mode does not send messages to a mobile network.</p>

        <section aria-labelledby="install-heading">
          <h2 id="install-heading">Installation</h2>
          <p><a :href="withBase('/downloads.html')">Download a binary</a> for your system, extract it, then run:</p>
          <pre tabindex="0" aria-label="Start command"><code>./textdock</code></pre>
          <p>Open <code>http://localhost:18257</code> in your browser.</p>
          <p>Docker and source builds are covered in the <a :href="withBase('/guide/getting-started.html')">installation guide</a>.</p>
        </section>

        <section aria-labelledby="message-heading">
          <h2 id="message-heading">Send a test message</h2>
          <pre tabindex="0" aria-label="Example SMS request"><code>curl http://localhost:18257/api/v1/messages \
  -H 'Content-Type: application/json' \
  -d '{"to":"+12025550123","from":"Acme","body":"Code 482193"}'</code></pre>
          <p>The message appears in the inbox. Add an authorization header if you enabled a server token.</p>
          <p>See the <a :href="withBase('/reference/api.html')">HTTP API reference</a> or use the <a :href="withBase('/reference/sdk.html')">Node SDK</a> in your application backend.</p>
        </section>

        <section aria-labelledby="guides-heading">
          <h2 id="guides-heading">Documentation</h2>
          <ul class="guide-list">
            <li><a :href="withBase('/guide/workflow.html')">Inboxes, search, CLI and backups</a></li>
            <li><a :href="withBase('/guide/connect.html')">Pair a phone with a read-only inbox</a></li>
            <li><a :href="withBase('/guide/test-numbers.html')">Test phone numbers and verification codes</a></li>
            <li><a :href="withBase('/guide/simulation.html')">Simulate delivery and inspect webhooks</a></li>
            <li><a :href="withBase('/guide/providers.html')">Provider compatibility and SDK configuration</a></li>
            <li><a :href="withBase('/guide/mobile-and-otp.html')">Native SMS and autofill limitations</a></li>
          </ul>
        </section>
      </div>

      <aside class="project-aside" aria-label="Project information">
        <h2>Release</h2>
        <p class="release-line"><a :href="release.url">{{ release.tag }}</a></p>
        <ul>
          <li><a :href="withBase('/downloads.html')">Downloads</a></li>
          <li><a :href="withBase('/project/changelog.html')">Changelog</a></li>
          <li><a href="https://github.com/fless-lab/TextDock">Source code</a></li>
          <li><a href="https://github.com/fless-lab/TextDock/issues">Issues</a></li>
        </ul>
        <h2>Project</h2>
        <ul>
          <li><a :href="withBase('/project/status.html')">Status and next steps</a></li>
          <li><a :href="withBase('/project/roadmap.html')">Roadmap</a></li>
          <li><a :href="withBase('/project/contributing.html')">Contributing</a></li>
          <li><a :href="withBase('/project/license.html')">MIT license</a></li>
          <li><a :href="withBase('/fr/projet.html')" lang="fr">Présentation en français</a></li>
        </ul>
        <p class="project-note">Pre-1.0 software.<br />Compatibility is documented per provider and operation.</p>
      </aside>
    </div>
  </main>
</template>
