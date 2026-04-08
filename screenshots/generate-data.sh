#!/usr/bin/env bash
# Generate fake session data for screenshots.
# Timestamps are relative to "now" so the picker always shows fresh relative times.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Helper: emit RFC 3339 timestamp offset from now.
# Usage: ts <minutes-ago>
ts() {
  if [[ "$(uname)" == "Darwin" ]]; then
    date -u -v-"${1}"M +"%Y-%m-%dT%H:%M:%SZ"
  else
    date -u -d "${1} minutes ago" +"%Y-%m-%dT%H:%M:%SZ"
  fi
}

HOME_DIR="/Users/drew.hays"
DEV="$HOME_DIR/Developer/github.com"

# --- OpenCode sessions ---
cat > "$SCRIPT_DIR/opencode.json" <<EOJSON
[
  {
    "id": "ses_a8f3c21d9ffe4kLmN2pQrStUvW",
    "title": "Add rate limiting middleware to atlas API",
    "slug": "bright-canyon",
    "created": "$(ts 45)",
    "last_used": "$(ts 8)",
    "directory": "$DEV/meridian-dev/atlas-api",
    "text": "Can you add rate limiting to our API? We need per-endpoint limits with Redis as the backend store. Start with the \`/auth\` endpoints since those are getting hammered.\n\nUse a sliding window algorithm and store counters in Redis. The middleware should go in \`middleware/ratelimit.go\`."
  },
  {
    "id": "ses_b7e2d10c8ffe3jKlM1oNpRsTuV",
    "title": "Fix connection pool exhaustion under load",
    "slug": "quiet-ember",
    "created": "$(ts 195)",
    "last_used": "$(ts 130)",
    "directory": "$DEV/meridian-dev/atlas-api",
    "text": "We're seeing connection pool exhaustion in production when traffic spikes. The pool is set to \`maxConns: 25\` but I don't think connections are being released properly in the database middleware.\n\nHere's the error we're seeing:\n\n\`\`\`\npq: sorry, too many clients already\n\`\`\`\n\nCan you look at \`db.Pool()\` usage in \`internal/database/pool.go\`?"
  },
  {
    "id": "ses_f3a8d76a4ffezFgHi7kJlNmOrQ",
    "title": "Debug flaky test in auth handler",
    "slug": "neon-meadow",
    "created": "$(ts 310)",
    "last_used": "$(ts 280)",
    "directory": "$DEV/meridian-dev/atlas-api",
    "text": "\`TestAuthHandler_RefreshToken\` is flaky in CI. It passes locally but fails about 30% of the time in GitHub Actions. I think it might be a timing issue with the token expiry check in \`validateRefreshToken()\`."
  },
  {
    "id": "ses_g2b1c09d5ffeWxYz6aBcDeFgHi",
    "title": "Add structured logging with slog",
    "slug": "golden-reef",
    "created": "$(ts 500)",
    "last_used": "$(ts 460)",
    "directory": "$DEV/meridian-dev/atlas-api",
    "text": "Replace the \`log\` package with \`log/slog\` throughout atlas-api. Add request ID to all log lines and make sure we're logging at the right levels. Errors should include stack context."
  },
  {
    "id": "ses_c6d1e09b7ffe2iJkL0nMoQrSuT",
    "title": "Set up GitHub Actions for deploy-tools",
    "slug": "silver-wolf",
    "created": "$(ts 1500)",
    "last_used": "$(ts 1440)",
    "directory": "$DEV/meridian-dev/deploy-tools",
    "text": "I want to set up CI for deploy-tools. We need lint, test, and a release workflow that builds binaries for linux and darwin. Use goreleaser for the release step."
  },
  {
    "id": "ses_h1a0b98c4ffeVwXy5zAbCdEfGh",
    "title": "Implement graceful shutdown for API server",
    "slug": "misty-peak",
    "created": "$(ts 2100)",
    "last_used": "$(ts 2040)",
    "directory": "$DEV/meridian-dev/atlas-api",
    "text": "The API server doesn't handle \`SIGTERM\` gracefully. In-flight requests get dropped during deploys. Add a shutdown handler that drains connections with a 30 second timeout."
  },
  {
    "id": "ses_d5c0f98a6ffe1hIjK9mLnPqRtS",
    "title": "Migrate user table to support multi-tenant auth",
    "slug": "iron-sparrow",
    "created": "$(ts 2900)",
    "last_used": "$(ts 2820)",
    "directory": "$DEV/meridian-dev/atlas-api",
    "text": "We need to add \`tenant_id\` to the \`users\` table and update the auth queries. Write a migration and update the repository layer."
  },
  {
    "id": "ses_i0z9a87b3ffeUvWx4yZaBcDeFg",
    "title": "Write integration tests for tenant onboarding flow",
    "slug": "rusty-comet",
    "created": "$(ts 3600)",
    "last_used": "$(ts 3500)",
    "directory": "$DEV/meridian-dev/atlas-api",
    "text": "We have no integration tests for the full tenant onboarding flow. Write tests that cover: create tenant, invite first admin user, admin sets up billing, first API key generated."
  },
  {
    "id": "ses_e4b9e87a5ffe0gHiJ8lKmOnPsR",
    "title": "Add shell completions for taskflow CLI",
    "slug": "dusty-orbit",
    "created": "$(ts 4400)",
    "last_used": "$(ts 4320)",
    "directory": "$DEV/dru89/taskflow",
    "text": "I want to add bash and zsh completions for taskflow. Use cobra's built-in completion generation."
  },
  {
    "id": "ses_j9y8z76a2ffeTuVw3xYzAbCdEf",
    "title": "Add retry logic to external API client",
    "slug": "pale-glacier",
    "created": "$(ts 6200)",
    "last_used": "$(ts 6100)",
    "directory": "$DEV/meridian-dev/atlas-api",
    "text": "The Stripe webhook client has no retry logic. Add exponential backoff with jitter for transient failures. Cap at 3 retries. Don't retry 4xx errors."
  },
  {
    "id": "ses_04a7c65a3ffeYeFgH6jIkMlNpP",
    "title": "Refactor config loading to support env overrides",
    "slug": "calm-ridge",
    "created": "$(ts 7300)",
    "last_used": "$(ts 7200)",
    "directory": "$DEV/dru89/taskflow",
    "text": "Taskflow's config loading is a mess. I want to support config files, env vars, and flags with proper precedence. Look at how viper handles this and see if we should just use it."
  },
  {
    "id": "ses_k8x7y65z1ffeStuV2wXyZaBcDe",
    "title": "Scaffold taskflow project with cobra",
    "slug": "bold-harbor",
    "created": "$(ts 9500)",
    "last_used": "$(ts 9400)",
    "directory": "$DEV/dru89/taskflow",
    "text": "I want to start a new CLI tool called taskflow for managing personal tasks from the terminal. Set up the Go module, add cobra for command handling, and create the basic add/list/done subcommands."
  },
  {
    "id": "ses_15a6b54a2ffeDdEfG5iHjLkMoO",
    "title": "Update tmux and neovim configs",
    "slug": "swift-lantern",
    "created": "$(ts 10100)",
    "last_used": "$(ts 10080)",
    "directory": "$DEV/dru89/dotfiles",
    "text": "Update my tmux config to use catppuccin theme and fix the neovim LSP setup for Go files. The gopls integration is broken after the last update."
  },
  {
    "id": "ses_l7w6x54y0ffeRstU1vWxYzAbCd",
    "title": "Add git aliases and delta pager config",
    "slug": "sharp-prism",
    "created": "$(ts 14400)",
    "last_used": "$(ts 14350)",
    "directory": "$DEV/dru89/dotfiles",
    "text": "Set up delta as my git pager with side-by-side diffs. Also add some aliases: git lg for a pretty log, git co for checkout, git fixup for quick amend."
  }
]
EOJSON

# --- Claude sessions ---
cat > "$SCRIPT_DIR/claude.json" <<EOJSON
[
  {
    "id": "01jf8k2m-3n4o-5p6q-7r8s-9t0u1v2w3x4y",
    "title": "Draft blog post on migrating from REST to gRPC",
    "slug": "gentle-harbor",
    "created": "$(ts 100)",
    "last_used": "$(ts 55)",
    "directory": "$DEV/dru89/blog",
    "text": "I'm writing a blog post about our experience migrating the atlas API from REST to gRPC. I want to cover the motivation, what went well, and the surprises. Audience is mid-senior engineers who are considering the same move."
  },
  {
    "id": "06jk3p7r-8s9t-0u1v-2w3x-4y5z6a7b8c9d",
    "title": "Explain atlas-api middleware chain to new team member",
    "slug": "warm-lantern",
    "created": "$(ts 420)",
    "last_used": "$(ts 390)",
    "directory": "$DEV/meridian-dev/atlas-api",
    "text": "I need to explain how our middleware chain works to a new team member. Walk through the request lifecycle from the router through auth, rate limiting, tenant context, and the handler. Focus on how context propagation works."
  },
  {
    "id": "02jg9l3n-4o5p-6q7r-8s9t-0u1v2w3x4y5z",
    "title": "Review atlas-web authentication flow",
    "slug": "copper-wave",
    "created": "$(ts 1600)",
    "last_used": "$(ts 1520)",
    "directory": "$DEV/meridian-dev/atlas-web",
    "text": "Walk me through the current auth flow in atlas-web. I want to understand the token refresh logic and where we're storing credentials. I think there might be a race condition when multiple tabs are open."
  },
  {
    "id": "07jl4q8s-9t0u-1v2w-3x4y-5z6a7b8c9d0e",
    "title": "Compare testing strategies for Go HTTP handlers",
    "slug": "silver-dune",
    "created": "$(ts 3200)",
    "last_used": "$(ts 3100)",
    "directory": "$DEV/meridian-dev/atlas-api",
    "text": "We're inconsistent about how we test HTTP handlers. Some tests use \`httptest.Server\`, some use \`httptest.ResponseRecorder\`, some mock the whole service layer. What's the right approach for our codebase?"
  },
  {
    "id": "03jh0m4o-5p6q-7r8s-9t0u-1v2w3x4y5z6a",
    "title": "Design doc for notification service architecture",
    "slug": "amber-cloud",
    "created": "$(ts 5800)",
    "last_used": "$(ts 5700)",
    "directory": "$DEV/meridian-dev/atlas-api",
    "text": "I need to write a design doc for a new notification service. It needs to support email, push, and in-app notifications with user preferences. Help me structure the doc and think through the tradeoffs between polling and WebSockets for in-app."
  },
  {
    "id": "08jm5r9t-0u1v-2w3x-4y5z-6a7b8c9d0e1f",
    "title": "Plan blog series on building multi-tenant SaaS",
    "slug": "frost-garden",
    "created": "$(ts 6800)",
    "last_used": "$(ts 6750)",
    "directory": "$DEV/dru89/blog",
    "text": "I want to write a 4-part blog series about building multi-tenant SaaS in Go. Help me outline the series: part 1 on data isolation strategies, part 2 on auth and tenant context, part 3 on billing integration, part 4 on operational lessons."
  },
  {
    "id": "04ji1n5p-6q7r-8s9t-0u1v-2w3x4y5z6a7b",
    "title": "Explore error boundary patterns for React 19",
    "slug": "velvet-prism",
    "created": "$(ts 8700)",
    "last_used": "$(ts 8640)",
    "directory": "$DEV/meridian-dev/atlas-web",
    "text": "React 19 changed how error boundaries work. What are the recommended patterns now? We have a custom ErrorBoundary component that catches render errors but I want to understand if there's a better approach with the new APIs."
  },
  {
    "id": "05jj2o6q-7r8s-9t0u-1v2w-3x4y5z6a7b8c",
    "title": "Outline Q3 platform reliability roadmap",
    "slug": "quiet-summit",
    "created": "$(ts 11500)",
    "last_used": "$(ts 11400)",
    "directory": "$HOME_DIR/Writing/Q3 Roadmap",
    "text": "Help me outline the Q3 reliability roadmap. Key themes are observability improvements, automated failover for the API tier, and SLO-based alerting. The audience is engineering leadership."
  }
]
EOJSON

# --- pi-mono sessions ---
cat > "$SCRIPT_DIR/pi-mono.json" <<EOJSON
[
  {
    "id": "pm_7x8y9z0a1b2c3d4e5f6g7h8i",
    "title": "Build dashboard charts with Recharts",
    "slug": "",
    "created": "$(ts 75)",
    "last_used": "$(ts 22)",
    "directory": "$DEV/meridian-dev/atlas-web",
    "text": "I need to add usage charts to the admin dashboard. We're using Recharts. I want a line chart for daily active users, a bar chart for API calls by endpoint, and a pie chart for error distribution. Pull data from the /analytics endpoint."
  },
  {
    "id": "pm_8y9z0a1b2c3d4e5f6g7h8i9j",
    "title": "Implement dark mode toggle in shared-ui",
    "slug": "",
    "created": "$(ts 350)",
    "last_used": "$(ts 290)",
    "directory": "$DEV/meridian-dev/shared-ui",
    "text": "Add dark mode support to the shared-ui component library. We need a ThemeProvider context, a toggle component, and CSS variables for the color tokens. All existing components should respect the theme."
  },
  {
    "id": "pm_2c3d4e5f6g7h8i9j0k1l2m3n",
    "title": "Add loading skeletons to dashboard pages",
    "slug": "",
    "created": "$(ts 700)",
    "last_used": "$(ts 650)",
    "directory": "$DEV/meridian-dev/atlas-web",
    "text": "The dashboard pages show a blank screen while data loads. Add skeleton loading states that match the layout of the real content. Use the Skeleton component from shared-ui."
  },
  {
    "id": "pm_9z0a1b2c3d4e5f6g7h8i9j0k",
    "title": "Fix table component sort not resetting on filter",
    "slug": "",
    "created": "$(ts 1700)",
    "last_used": "$(ts 1650)",
    "directory": "$DEV/meridian-dev/shared-ui",
    "text": "The DataTable component has a bug where the sort order persists when you change the filter. When a user filters the table, the sort should reset to the default. Also the sort indicator arrow is rendering wrong in Safari."
  },
  {
    "id": "pm_3d4e5f6g7h8i9j0k1l2m3n4o",
    "title": "Build tenant settings page with form validation",
    "slug": "",
    "created": "$(ts 2500)",
    "last_used": "$(ts 2400)",
    "directory": "$DEV/meridian-dev/atlas-web",
    "text": "Build the tenant settings page. It needs sections for general info, billing, and team members. Use react-hook-form for validation. The billing section should show current plan and usage."
  },
  {
    "id": "pm_0a1b2c3d4e5f6g7h8i9j0k1l",
    "title": "Add search with keyboard navigation to atlas-web",
    "slug": "",
    "created": "$(ts 4500)",
    "last_used": "$(ts 4400)",
    "directory": "$DEV/meridian-dev/atlas-web",
    "text": "Build a command palette style search for atlas-web. Cmd+K to open, fuzzy search over pages and recent items, arrow keys to navigate results, enter to select. Use the cmdk library."
  },
  {
    "id": "pm_4e5f6g7h8i9j0k1l2m3n4o5p",
    "title": "Add toast notification system to shared-ui",
    "slug": "",
    "created": "$(ts 5500)",
    "last_used": "$(ts 5400)",
    "directory": "$DEV/meridian-dev/shared-ui",
    "text": "We need a toast notification component in shared-ui. Support success, error, warning, and info variants. Auto-dismiss after 5 seconds with a progress bar. Stack multiple toasts vertically."
  },
  {
    "id": "pm_1b2c3d4e5f6g7h8i9j0k1l2m",
    "title": "Migrate atlas-web from webpack to vite",
    "slug": "",
    "created": "$(ts 8800)",
    "last_used": "$(ts 8600)",
    "directory": "$DEV/meridian-dev/atlas-web",
    "text": "Our webpack build takes 45 seconds. Let's migrate to vite. The main concerns are our custom webpack plugins for SVG handling and the proxy config for the API dev server."
  },
  {
    "id": "pm_5f6g7h8i9j0k1l2m3n4o5p6q",
    "title": "Set up Storybook for shared-ui components",
    "slug": "",
    "created": "$(ts 12000)",
    "last_used": "$(ts 11900)",
    "directory": "$DEV/meridian-dev/shared-ui",
    "text": "Set up Storybook for the shared-ui library. Create stories for Button, Input, DataTable, Modal, and the new toast component. Configure it to use our theme tokens."
  }
]
EOJSON

# --- omp sessions ---
cat > "$SCRIPT_DIR/omp.json" <<EOJSON
[
  {
    "id": "omp_k3l4m5n6o7p8q9r0s1t2u3v4",
    "title": "Add health check endpoint with dependency status",
    "slug": "",
    "created": "$(ts 160)",
    "last_used": "$(ts 120)",
    "directory": "$DEV/meridian-dev/atlas-api",
    "text": "Add a \`/healthz\` endpoint that checks Redis, Postgres, and the notification service. Return \`200\` if all healthy, \`503\` if any are down. Include individual dependency status in the response body."
  },
  {
    "id": "omp_q9r0s1t2u3v4w5x6y7z8a9b0",
    "title": "Write OpenAPI spec for atlas-api v2 endpoints",
    "slug": "",
    "created": "$(ts 800)",
    "last_used": "$(ts 740)",
    "directory": "$DEV/meridian-dev/atlas-api",
    "text": "Document the v2 API endpoints in OpenAPI 3.1 format. Cover tenants, users, API keys, and the analytics endpoints. Include request/response examples and error schemas."
  },
  {
    "id": "omp_l4m5n6o7p8q9r0s1t2u3v4w5",
    "title": "Write Terraform module for staging environment",
    "slug": "",
    "created": "$(ts 1800)",
    "last_used": "$(ts 1750)",
    "directory": "$DEV/meridian-dev/deploy-tools",
    "text": "I need a Terraform module that provisions a staging environment. It should create an RDS instance, ElastiCache Redis, ECS service, and ALB. Parameterize the instance sizes so we can use smaller ones for staging."
  },
  {
    "id": "omp_r0s1t2u3v4w5x6y7z8a9b0c1",
    "title": "Add Makefile targets for common dev tasks",
    "slug": "",
    "created": "$(ts 2600)",
    "last_used": "$(ts 2550)",
    "directory": "$DEV/meridian-dev/atlas-api",
    "text": "Create a Makefile with targets for build, test, lint, migrate, and run. Add a docker-compose up target for local dependencies. Include a help target that lists everything."
  },
  {
    "id": "omp_m5n6o7p8q9r0s1t2u3v4w5x6",
    "title": "Set up Datadog APM tracing for atlas services",
    "slug": "",
    "created": "$(ts 2950)",
    "last_used": "$(ts 2880)",
    "directory": "$DEV/meridian-dev/atlas-api",
    "text": "Integrate Datadog APM tracing into atlas-api. I want distributed traces across HTTP handlers, database queries, and Redis calls. Use dd-trace-go and make sure we're propagating trace context to downstream services."
  },
  {
    "id": "omp_s1t2u3v4w5x6y7z8a9b0c1d2",
    "title": "Build deploy script with blue-green rollout",
    "slug": "",
    "created": "$(ts 4800)",
    "last_used": "$(ts 4700)",
    "directory": "$DEV/meridian-dev/deploy-tools",
    "text": "Write a deploy script that does blue-green deployments to ECS. It should deploy to the inactive target group, run health checks, then swap the ALB listener. Include a rollback flag."
  },
  {
    "id": "omp_n6o7p8q9r0s1t2u3v4w5x6y7",
    "title": "Create Homebrew formula for taskflow",
    "slug": "",
    "created": "$(ts 5900)",
    "last_used": "$(ts 5850)",
    "directory": "$DEV/dru89/taskflow",
    "text": "I want to publish taskflow via Homebrew. Set up a tap repository and write the formula. The binary is built with goreleaser and hosted on GitHub releases."
  },
  {
    "id": "omp_o7p8q9r0s1t2u3v4w5x6y7z8",
    "title": "Debug slow query on tenant dashboard endpoint",
    "slug": "",
    "created": "$(ts 7400)",
    "last_used": "$(ts 7300)",
    "directory": "$DEV/meridian-dev/atlas-api",
    "text": "The \`GET /tenants/:id/dashboard\` endpoint is taking 3-4 seconds. Run \`EXPLAIN ANALYZE\` on the queries and suggest index changes. I suspect the join on \`activity_log\` is the bottleneck."
  },
  {
    "id": "omp_p8q9r0s1t2u3v4w5x6y7z8a9",
    "title": "Fix fzf keybindings conflicting with tmux",
    "slug": "",
    "created": "$(ts 10200)",
    "last_used": "$(ts 10150)",
    "directory": "$DEV/dru89/dotfiles",
    "text": "fzf's Ctrl-T binding is conflicting with my tmux prefix. I want to rebind fzf to use Ctrl-F instead, and update the preview window to use bat for syntax highlighting."
  },
  {
    "id": "omp_t2u3v4w5x6y7z8a9b0c1d2e3",
    "title": "Set up pre-commit hooks for atlas-api",
    "slug": "",
    "created": "$(ts 13000)",
    "last_used": "$(ts 12900)",
    "directory": "$DEV/meridian-dev/atlas-api",
    "text": "Add pre-commit hooks to atlas-api: \`go vet\`, \`staticcheck\`, \`gofumpt\` formatting check, and a check that migration files have a down migration. Use \`lefthook\` or \`pre-commit\` framework."
  }
]
EOJSON

echo "Generated session data in $SCRIPT_DIR/"
echo "  opencode.json  ($(python3 -c "import json; print(len(json.load(open('$SCRIPT_DIR/opencode.json'))))" 2>/dev/null) sessions)"
echo "  claude.json    ($(python3 -c "import json; print(len(json.load(open('$SCRIPT_DIR/claude.json'))))" 2>/dev/null) sessions)"
echo "  pi-mono.json   ($(python3 -c "import json; print(len(json.load(open('$SCRIPT_DIR/pi-mono.json'))))" 2>/dev/null) sessions)"
echo "  omp.json       ($(python3 -c "import json; print(len(json.load(open('$SCRIPT_DIR/omp.json'))))" 2>/dev/null) sessions)"
