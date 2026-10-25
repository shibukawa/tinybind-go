package httpbind

import (
	"io"
	"net/http"
	"strconv"
	"strings"
)

// SwaggerUI returns an http.Handler that serves a minimal Swagger UI page
// loading the OpenAPI document from specURL (e.g. "/openapi.json").
//
// Assets are loaded from a public CDN; this handler does not embed Swagger UI
// binaries. Mount freely, e.g.:
//
//	mux.Handle("GET /docs/{$}", httpbind.SwaggerUI("/openapi.json"))
func SwaggerUI(specURL string) http.Handler {
	if strings.TrimSpace(specURL) == "" {
		specURL = "/openapi.json"
	}
	page := strings.Replace(swaggerUIPage, "__SPEC_URL__", safeJSString(specURL), 1)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, page)
	})
}

// safeJSString quotes a JavaScript string and escapes HTML-significant runes so
// an untrusted specURL cannot terminate the inline script element.
func safeJSString(s string) string {
	return strings.NewReplacer(
		"<", `\u003c`, ">", `\u003e`, "&", `\u0026`,
		"\u2028", `\u2028`, "\u2029", `\u2029`,
	).Replace(strconv.Quote(s))
}

const swaggerUIPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>httpbind API docs</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui.css"/>
  <style>
    body { margin: 0; background: #fafafa; }
    #swagger-ui { max-width: 100%; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-standalone-preset.js"></script>
  <script>
    window.onload = function () {
      window.ui = SwaggerUIBundle({
        url: __SPEC_URL__,
        dom_id: "#swagger-ui",
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
        layout: "StandaloneLayout",
        tryItOutEnabled: true
      });
    };
  </script>
</body>
</html>
`
