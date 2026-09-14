export async function copyText(text: string): Promise<boolean> {
  if (navigator.clipboard) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      /* LAN fallback below. */
    }
  }
  const previous = document.activeElement as HTMLElement | null;
  const input = document.createElement("textarea");
  input.value = text;
  input.className = "clipboard-fallback";
  input.setAttribute("aria-hidden", "true");
  // A modal traps focus, so insert the fallback in the active dialog if present.
  (document.querySelector("dialog[open]") || document.body).append(input);
  input.select();
  let copied = false;
  try {
    copied = document.execCommand("copy");
  } catch {
    /* Selectable text remains available. */
  }
  input.remove();
  previous?.focus({ preventScroll: true });
  return copied;
}
