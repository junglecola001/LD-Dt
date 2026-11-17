import DOMPurify from 'dompurify';

const ALLOWED_TAGS = [
  'p',
  'br',
  'strong',
  'em',
  'del',
  'code',
  'pre',
  'blockquote',
  'ul',
  'ol',
  'li',
  'a',
  'img',
  'h1',
  'h2',
  'h3',
  'h4',
  'table',
  'thead',
  'tbody',
  'tr',
  'th',
  'td',
  'hr',
  'span',
  'div',
];

const ALLOWED_ATTR = ['href', 'src', 'alt', 'title', 'class', 'target', 'rel', 'width', 'height'];

let hooksRegistered = false;

function ensureHooks(): void {
  if (hooksRegistered) return;
  hooksRegistered = true;

  DOMPurify.addHook('afterSanitizeAttributes', (node) => {
    if (node.tagName === 'A') {
      const href = node.getAttribute('href') ?? '';
      if (!/^https?:\/\//i.test(href) && !href.startsWith('/')) {
        node.removeAttribute('href');
      } else {
        node.setAttribute('rel', 'noopener noreferrer');
        node.setAttribute('target', '_blank');
      }
    }
    if (node.tagName === 'IMG') {
      const src = node.getAttribute('src') ?? '';
      if (!/^https?:\/\//i.test(src) && !src.startsWith('data:image/')) {
        node.removeAttribute('src');
      }
      node.setAttribute('loading', 'lazy');
    }
  });
}

/**
 * Sanitizes Discourse cooked HTML using an explicit tag/attribute allowlist (plan.md 8.1).
 * Blocks scripts, event handlers, iframes, and non-HTTP(S) URL schemes.
 */
export function sanitizeCookedHTML(dirty: string): string {
  ensureHooks();
  return DOMPurify.sanitize(dirty, {
    ALLOWED_TAGS,
    ALLOWED_ATTR,
    ALLOW_DATA_ATTR: false,
    FORBID_TAGS: ['script', 'style', 'iframe', 'object', 'embed', 'svg', 'math', 'form'],
    FORBID_ATTR: ['style', 'onerror', 'onload', 'onclick', 'onmouseover'],
  });
}
