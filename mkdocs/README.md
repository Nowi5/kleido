# Kleido Documentation

Built with [MkDocs](https://www.mkdocs.org/) + [Material for MkDocs](https://squidfunk.github.io/mkdocs-material/).

The docs folder sits at the **repo root** (`kleido/docs/`) alongside `myapp/`. The Docker service that serves it is defined in the **root-level** `kleido/docker-compose.yml`.

---

## Viewing the docs

### Option A — Docker (no Python required)

Run from the **repo root** (`kleido/`):

```bash
docker-compose up -d docs
# Open: http://localhost:8000
```

Live-reload is built in — edit any `.md` file and the browser refreshes automatically.

To build a static site instead of serving:

```bash
docker-compose run --rm docs build
# Output written to docs/site/
```

### Option B — Local Python

Run from the `docs/` directory:

```bash
# Linux / macOS
pip install mkdocs-material   # once
mkdocs serve                  # http://localhost:8000

# Windows — if mkdocs is not on PATH:
python -m pip install mkdocs-material
python -m mkdocs serve
```

!!! note "Windows Python path"
    If `pip` is not found, locate Python and invoke it directly:
    ```bash
    /c/Users/$USERNAME/AppData/Local/Python/bin/python.exe -m pip install mkdocs-material
    python -m mkdocs serve
    ```

---

## Updating the docs

All content is Markdown in `docs/content/`. Edit any file while `mkdocs serve` is running and the browser auto-reloads.

### Adding a new page

1. Create `docs/content/<page>.md`
2. Add an entry to the `nav:` section in `docs/mkdocs.yml`
3. The dev server picks it up instantly — no restart needed

### Page inventory

| File | Content |
|------|---------|
| `index.md` | Home — service map, architecture, request lifecycle |
| `getting-started.md` | Setup guide (Docker + Windows + local dev) |
| `swagger.md` | Swagger UI walkthrough |
| `auth-users.md` | Auth & Users API — full curl examples and error reference |
| `jaeger.md` | Distributed tracing guide |
| `prometheus.md` | Metrics catalogue and PromQL cookbook |
| `grafana.md` | Dashboard panels and Grafana tips |

---

## Folder structure

```
kleido/                          ← repo root
├── docker-compose.yml           ← root compose: include myapp stack + docs service
├── docs/                        ← this folder
│   ├── mkdocs.yml               ← site config, nav, theme
│   ├── content/                 ← Markdown source pages
│   │   ├── index.md
│   │   ├── getting-started.md
│   │   ├── swagger.md
│   │   ├── auth-users.md
│   │   ├── jaeger.md
│   │   ├── prometheus.md
│   │   └── grafana.md
│   ├── .gitignore               ← excludes site/ and .venv/
│   └── README.md                ← this file
└── myapp/                       ← Go application
    └── docker-compose.yml       ← app stack only (no docs service)
```

The root `docker-compose.yml` uses the Compose `include:` directive to pull in `myapp/docker-compose.yml` and appends the `docs` service. This keeps both files focused and avoids cross-directory volume paths.

---

## Deploy to GitHub Pages

Uncomment `site_url` in `mkdocs.yml` first (set it to your Pages URL), then:

```bash
# From docs/
mkdocs gh-deploy
```

This builds the site and force-pushes it to the `gh-pages` branch automatically. The `site/` directory is in `.gitignore` and never committed to `main`.
