package controller

import "html/template"

const oauthAuthorizeBrandName = "锐捷TokenHub"

type oauthAuthorizationTemplateData struct {
	IsCodexClient bool
	ClientName    string
	ClientID      string
	Scopes        []string
	RedirectURI   string
	State         string
	Nonce         string
	Challenge     string
	Method        string
}

var oauthAuthorizeTemplate = template.Must(template.New("oauth_authorize").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{if .IsCodexClient}}锐捷Codex 认证{{else}}授权 {{.ClientName}}{{end}} — ` + oauthAuthorizeBrandName + `</title>
  <style>
    :root {
      color-scheme: light;
      --blue: #0868f7;
      --blue-dark: #0054d8;
      --ink: #0b0d11;
      --text: #1b2029;
      --muted: #626b7a;
      --line: #e5e9f0;
      --soft: #f5f8fd;
      --white: #fff;
      --focus: rgba(8, 104, 247, .28);
      --font: "PingFang SC", "Microsoft YaHei", "Noto Sans SC", system-ui, -apple-system, sans-serif;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    html { height: 100%; }
    body {
      min-height: 100%;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 24px;
      color: var(--text);
      background: var(--soft);
      font-family: var(--font);
      -webkit-font-smoothing: antialiased;
      text-rendering: optimizeLegibility;
    }
    .card {
      width: 100%;
      max-width: 460px;
      overflow: hidden;
      background: var(--white);
      border-radius: 18px;
      box-shadow: 0 28px 90px rgba(9, 21, 43, .12), 0 2px 8px rgba(9, 21, 43, .06);
    }
    .card-header {
      padding: 40px 36px 0;
      text-align: center;
    }
    .brand-name {
      display: inline-block;
      margin-bottom: 28px;
      color: var(--ink);
      font-size: 18px;
      font-weight: 700;
      letter-spacing: -.02em;
    }
    h1 {
      margin-bottom: 10px;
      color: var(--ink);
      font-size: 26px;
      font-weight: 700;
      line-height: 1.3;
      letter-spacing: -.02em;
    }
    .subtitle {
      color: var(--muted);
      font-size: 14px;
      line-height: 1.7;
    }
    .card-body { padding: 30px 36px 34px; }
    .scope-card {
      margin-bottom: 22px;
      padding: 20px;
      background: var(--soft);
      border: 1px solid var(--line);
      border-radius: 12px;
    }
    .scope-label {
      margin-bottom: 14px;
      color: var(--muted);
      font-size: 12px;
      font-weight: 650;
      letter-spacing: .06em;
    }
    .scope-list {
      display: grid;
      gap: 10px;
      padding-left: 20px;
    }
    .scope-list li {
      color: var(--text);
      font-size: 14px;
      line-height: 1.5;
    }
    .scope-list li::marker { color: var(--blue); }
    .redirect-info,
    .redirect-details {
      margin-bottom: 24px;
      color: var(--muted);
      font-size: 12px;
      line-height: 1.6;
    }
    .redirect-info span,
    .redirect-value {
      color: var(--text);
      word-break: break-all;
    }
    .redirect-details {
      padding: 12px 14px;
      background: var(--soft);
      border: 1px solid var(--line);
      border-radius: 10px;
    }
    .redirect-details summary {
      color: var(--muted);
      cursor: pointer;
      font-weight: 600;
      outline: none;
    }
    .redirect-details summary:focus-visible {
      border-radius: 4px;
      box-shadow: 0 0 0 3px var(--focus);
    }
    .redirect-value { padding-top: 8px; }
    .actions {
      display: flex;
      gap: 12px;
    }
    button {
      min-height: 48px;
      flex: 1;
      padding: 0 20px;
      border: 1px solid transparent;
      border-radius: 10px;
      font-family: var(--font);
      font-size: 15px;
      font-weight: 650;
      cursor: pointer;
      transition: transform .18s ease, background .18s ease, border-color .18s ease, box-shadow .18s ease;
    }
    button:hover { transform: translateY(-1px); }
    button:active { transform: translateY(0); }
    button:focus-visible {
      outline: none;
      box-shadow: 0 0 0 4px var(--focus);
    }
    .approve {
      color: #fff;
      background: var(--blue);
      box-shadow: 0 8px 20px rgba(8, 104, 247, .18);
    }
    .approve:hover { background: var(--blue-dark); }
    .approve:focus-visible { box-shadow: 0 0 0 4px var(--focus), 0 8px 20px rgba(8, 104, 247, .18); }
    .deny {
      color: var(--text);
      background: var(--white);
      border-color: var(--line);
    }
    .deny:hover { border-color: #9aa2af; }
    .card-footer {
      padding: 0 36px 26px;
      text-align: center;
    }
    .security-note {
      color: var(--muted);
      font-size: 12px;
    }
    @media (max-width: 480px) {
      body { padding: 16px; }
      .card-header { padding: 32px 22px 0; }
      .card-body { padding: 26px 22px 30px; }
      .card-footer { padding: 0 22px 22px; }
      .brand-name { margin-bottom: 22px; }
      h1 { font-size: 23px; }
    }
  </style>
</head>
<body>
<main class="card">
  <header class="card-header">
    <div class="brand-name">` + oauthAuthorizeBrandName + `</div>
    {{if .IsCodexClient}}
    <h1>锐捷Codex 认证</h1>
    <p class="subtitle">授权后，锐捷Codex 将使用你的锐捷TokenHub 账户。</p>
    {{else}}
    <h1>授权 {{.ClientName}}</h1>
    <p class="subtitle">{{.ClientName}} 正在申请访问你的账户。</p>
    {{end}}
  </header>
  <div class="card-body">
    {{if .IsCodexClient}}
    <details class="redirect-details">
      <summary>查看认证回调地址</summary>
      <div class="redirect-value">{{.RedirectURI}}</div>
    </details>
    {{else}}
    <section class="scope-card" aria-labelledby="scope-label">
      <div class="scope-label" id="scope-label">申请以下权限</div>
      <ul class="scope-list">
        {{range .Scopes}}<li>{{.}}</li>{{end}}
      </ul>
    </section>
    <div class="redirect-info">认证回调地址：<span>{{.RedirectURI}}</span></div>
    {{end}}
    <form method="post" action="/oauth/authorize">
      <input type="hidden" name="response_type" value="code">
      <input type="hidden" name="client_id" value="{{.ClientID}}">
      <input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
      <input type="hidden" name="scope" value="{{range $i, $s := .Scopes}}{{if $i}} {{end}}{{$s}}{{end}}">
      <input type="hidden" name="state" value="{{.State}}">
      <input type="hidden" name="nonce" value="{{.Nonce}}">
      <input type="hidden" name="code_challenge" value="{{.Challenge}}">
      <input type="hidden" name="code_challenge_method" value="{{.Method}}">
      <div class="actions">
        {{if not .IsCodexClient}}<button class="deny" type="submit" name="decision" value="deny">拒绝</button>{{end}}
        <button class="approve" type="submit" name="decision" value="approve">授权</button>
      </div>
    </form>
  </div>
  <footer class="card-footer">
    <div class="security-note">受 OAuth 2.0 安全保护</div>
  </footer>
</main>
</body>
</html>`))
