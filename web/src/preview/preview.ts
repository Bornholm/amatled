import { rpc } from "../bridge";

const ANCHOR_SCRIPT = `
<script>
document.addEventListener('click', function(e) {
  var a = e.target && e.target.closest('a');
  if (!a) return;
  var href = a.getAttribute('href');
  if (!href) return;
  e.preventDefault();
  if (href.startsWith('#')) {
    var id = href.slice(1);
    var el = document.getElementById(id) || document.querySelector('[name="' + id + '"]');
    if (el) el.scrollIntoView({ behavior: 'smooth' });
  }
});
</script>
`;

// Surcharge CSS sombre injectée dans le HTML rendu par amatl
const DARK_OVERRIDE = `
body {
  background: #1e1e1e !important;
  color: #d4d4d4 !important;
}
pre, code {
  background: #252526 !important;
  border-radius: 4px;
}
a { color: #4ec9b0; }
h1, h2, h3, h4, h5, h6 { color: #9cdcfe; }
blockquote {
  border-left: 3px solid #3e3e42;
  padding-left: 0.75em;
  color: #858585;
}
table { border-collapse: collapse; }
th, td {
  border: 1px solid #3e3e42;
  padding: 4px 8px;
}
th { background: #252526; }
hr { border-color: #3e3e42; }
`;

export class Preview {
  constructor(
    private frame: HTMLIFrameElement,
    private spinner: HTMLElement
  ) {}

  async render(fileId: string): Promise<void> {
    this.spinner.classList.remove("hidden");
    this.frame.srcdoc = "";
    try {
      const { html } = await rpc<{ html: string }>("document.render", {
        fileId,
      });
      this.frame.srcdoc = this.injectDark(html);
    } catch (err) {
      this.frame.srcdoc = `<!DOCTYPE html><html><body style="background:#1e1e1e;color:#f44747;padding:1rem;font-family:monospace">
<strong>Erreur de rendu amatl</strong><pre>${String(err)}</pre></body></html>`;
    } finally {
      this.spinner.classList.add("hidden");
    }
  }

  private injectDark(html: string): string {
    const tag = `<style>${DARK_OVERRIDE}</style>` + ANCHOR_SCRIPT;
    if (html.includes("</head>")) {
      return html.replace("</head>", tag + "\n</head>");
    }
    return tag + html;
  }
}
