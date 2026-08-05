# htmlbind セキュリティリファレンス

文字列を HTML escape すれば安全、とは限りません。`javascript:alert(1)` には `&` も `<` も `>` も `"` も `'` も含まれないので、HTML escape はこれをそのまま返します。テキストとしてはそれで正しく、同じ値が `href` に入ればスクリプト実行になります。

そのため `htmlbind` は、値そのものではなく**値が書かれた位置**で扱いを決めます。この文書は、位置ごとに何を受け付け、何を書き換え、どこまでで守備範囲が終わるのかをまとめたものです。

テンプレート言語そのものについては [htmlbind.ja.md](htmlbind.ja.md) を参照してください。

## 位置ごとの規則

| 位置 | 受け付ける型 | 描画時の処理 |
| --- | --- | --- |
| HTML テキスト | 任意 | HTML escape |
| 通常の属性 | `html` と trusted 型以外 | HTML escape |
| URL 属性 | `url` のみ | スキーム検査 → HTML escape |
| URL リスト属性 | `string` | 要素ごとのスキーム検査 → HTML escape |
| イベントハンドラ属性 | `trusted_javascript` のみ | HTML escape |
| `<script>` の内容 | `trusted_javascript` / `script_json` | なし |
| `<style>` の内容 | `trusted_css` | なし |

このうち URL 属性の行だけは検査が二段になっています。生成時に `string` を弾き、描画時にスキームを見る。型システムが保証できるのは「`url.URL` としてパースを通った」ことまでで、「まともな行き先を指している」ことではないからです。

## URL 属性

URL 属性は `url` を要求します。`string` を渡すと生成が失敗します。

```text
export component Bad(link: string): html {
<a href={link}>x</a>
}
```

```
attribute href requires url, got string
```

この検査は描画前に実際の仕事をしています。`url.Parse` は古典的な難読化のいくつかをその場で拒否するからです。`java\tscript:alert(1)` は制御文字で失敗し、`" javascript:alert(1)"` は「URL の最初のパスセグメントにコロンは置けない」という理由で失敗します。`url.URL` として描画まで到達した値は、すでにパースを通り抜けています。

ただしパースは行き先を判断しません。`url.Parse("javascript:alert(1)")` は成功します。だからスキームは描画時にもう一度見られます。

### 描画されるスキーム

既定では、URL 属性は以下だけを描画します。

| 形 | 例 | 出力 |
| --- | --- | --- |
| `http` / `https` | `https://example.com/a` | そのまま |
| `mailto` | `mailto:a@example.com` | そのまま |
| `tel` | `tel:+81-3-0000-0000` | そのまま |
| 相対 | `/images/logo.png` | そのまま |
| スキーム相対 | `//cdn.example.com/x.js` | そのまま |
| フラグメント | `#section` | そのまま |
| ラスタ画像の `data:` | `data:image/png;base64,…` | そのまま |
| それ以外 | `javascript:alert(1)` | `#tb-blocked-url` |

スキームを持たない値は常に通ります。文書自身のオリジンに対して解決されるだけで、別のプロトコルには届かないので、名簿の対象外です。

拒否された値は、落とすのではなく置き換えます。

```html
<a href="#tb-blocked-url">…</a>
```

属性を残すのは意図的です。`href` ごと落としてしまうと、テンプレートが最初から書かなかった `href` と見分けがつきません。誤って弾かれた URL を追う手がかりが何も残らなくなります。`#tb-blocked-url` はフラグメントなので、現在の文書に解決されるだけで、どこにも到達しません。

### 何を読んで判定しているか

スキームは、パース済み URL の `Scheme` フィールドではなく、**ブラウザが解決する文字列**から読みます。この違いは見た目より効きます。

```go
hostile := url.URL{Opaque: "javascript:alert(1)"}
hostile.Scheme        // ""
hostile.String()      // "javascript:alert(1)"
```

構造体を信じる検査からは、`Scheme` が空なので「相対 URL」に見えます。それでいて `String()` はブラウザが実行するものを返します。逆向きの穴もあります。`url.URL{Scheme: "JAVASCRIPT"}` は大文字のままです。小文字化は `url.Parse` の仕事で、構造体への代入は通らないからです。

ブラウザは URL をパースする前にタブ・改行・復帰を取り除きます。ブラウザから見れば `java\tscript:` は `javascript:` URL です。フィルタも同じ文字を取り除き、先頭の制御文字を落としてからスキームを読みます。

### インライン画像

`data:` はスキーム単位で許可・拒否するのではなく、メディアタイプで判定します。インライン画像は普通のオーサリングだからです。

| メディアタイプ | 既定 |
| --- | --- |
| `image/png` `image/jpeg` `image/gif` `image/webp` `image/avif` `image/bmp` `image/x-icon` | 描画 |
| `image/svg+xml` | 遮断 |
| `text/html` とその他すべて | 遮断 |

SVG が許可側にないのは意図的です。SVG 文書はスクリプトを持てるので、`data:image/svg+xml` は画像のメディアタイプをまとったスクリプトシンクになります。必要で、かつその文書の出所を自分で管理できるなら、`WithDataURLMediaTypes` で足してください。

### 属性の全名簿

以下はすべて `url` を要求し、スキーム検査を通ります。

```
href  src  action  formaction  poster  data  xlink:href
cite  background  longdesc  manifest
classid  codebase  archive  profile
```

スクリプト実行の危険があるのは最初の group だけです。残りが名簿にいる理由は別で、ブラウザがそれらを解決する以上、素の `string` から任意の行き先を指せる状態にしておきたくない、というものです。`manifest` `classid` `codebase` `archive` `profile` が駆動していた機構はブラウザから消えています。名簿に入れるコストがほぼゼロで、以後この問いが再燃しないというだけの理由で入っています。

### リスト値の属性

`srcset` `imagesrcset` `ping` は複数の URL を持ちます。単一の `url.URL` では表現できないので、`string` のまま、描画時に要素ごとに検査されます。

```text
srcset={candidates}
```

```
/a.png 1x, javascript:alert(1) 2x, /b.png 3x
```

は次のように出力されます。

```html
srcset="/a.png 1x, /b.png 3x"
```

拒否された候補だけが落ちます。属性ごと拒否すると、敵対的な候補が1つ混ざっただけで画像が消えることになるからです。

ただし、ここで何が守られていないかは知っておいてください。これらの属性が抱えるリスクは、たいてい敵対的なスキームではありません。`ping="https://attacker.example/collect"` は正当な `https` URL で、リンクがクリックされればブラウザはそこへ POST します。スキーム検査はそれについて何の意見も持ちません。`ping` や `srcset` にユーザー由来の値を入れるなら、それは「自分で信頼すると決めた行き先」として扱ってください。

## イベントハンドラ属性

イベントハンドラの値はブラウザが JavaScript としてコンパイルします。そのため `on` で始まる属性は `trusted_javascript` だけを受け付けます。

```text
export component Bad(value: string): html {
<button onclick={value}>x</button>
}
```

```
html:event requires trusted_javascript; wrap the value in RawJavaScript to state
that it is code, or attach the behavior with server-action instead
```

ここには検査すべきスキームがありません。属性値の全体がハンドラ本体です。だからこの規則は何かを書き換えるのではなく、型を正直にする方向を取っています。

```text
<button onclick={RawJavaScript(code)}>x</button>
```

`RawJavaScript` は信頼の表明であって、サニタイザではありません。振る舞いを付ける経路として推奨されるのは従来どおり `server-action` です。Go の関数名を静的に指すので、クライアントコードに値が差し込まれることがありません。

影響を受けないものが2つあります。挿入を含まない静的なハンドラは書かれたままのマークアップで、そのまま通ります。`on-click` のようなハイフン付きの名前は、イベントハンドラの語彙ではなくカスタム要素のものなので、対象外です。

```text
<button onclick="doThing()">x</button>
<p on-click={value}>x</p>
```

ハンドラに到達した値は、それでも HTML escape されます。矛盾ではありません。HTML パーサは属性値をデコードしてからハンドラ本体をコンパイルするので、escape は値をクォートの内側に留めるだけで、ブラウザが読む JavaScript は変わりません。

## 設定

どちらのポリシーも描画オプションです。1つのバイナリが2つのアプリケーションを別々の規則で動かせます。

```go
htmlbind.Render(w, page,
    htmlbind.WithURLSchemes("https", "mailto", "sms"),
    htmlbind.WithDataURLMediaTypes("image/png", "image/svg+xml"),
)
```

各オプションは既定集合に**追加**するのではなく**置き換え**ます。引数なしで呼べば何も許可しません。これは使えるポリシーで、オプションを設定していない状態とは区別されます。

```go
htmlbind.WithURLSchemes()          // 相対 URL だけが描画される
htmlbind.WithDataURLMediaTypes()   // data URL は一切描画されない
```

相対・スキーム相対・フラグメントは設定に関係なく描画されます。

| シンボル | 意味 |
| --- | --- |
| `htmlbind.DefaultURLSchemes` | `http` `https` `mailto` `tel` |
| `htmlbind.DefaultDataURLMediaTypes` | 上記のラスタ画像メディアタイプ |
| `htmlbind.BlockedURL` | `#tb-blocked-url`、置換後の値 |

既定を描画ごとに広げられるからこそ、既定そのものは意図的に狭くしてあります。`ftp` や自前の登録済みスキームが要るアプリケーションがそう言えばよく、全員に緩い既定を配るより安く済みます。

## 既知の未対応

### meta refresh

`<meta http-equiv="refresh" content="0;url=…">` はページを遷移させますが、この URL はフィルタされません。

属性名の名簿では見つけられないためです。属性の名前は `content` で、それが URL の意味を持つのは兄弟の `http-equiv="refresh"` があるときだけです。照合するには属性名ではなく要素と第2の属性を読む必要があり、その実装はまだ入っていません。

ここでのリスクは敵対的なスキームではありません。meta refresh の `javascript:` はブラウザがすでに拒否します。危ないのは普通の `https` の行き先です。

```html
<meta http-equiv="refresh" content="0;url=https://attacker.example/">
```

これはクリックも操作もなしに遷移します。この位置が塞がるまで、meta refresh の `content` を信頼できない入力から組み立てないでください。

### redraw パラメータのデコード

`htmlupdate.QueryURL` は redraw パラメータを `url.Parse` でデコードし、パースが通ったスキームをすべて受け付けます。描画側はフィルタされているので、この経路で来た敵対的なスキームは実行される場所で無害化されます。ただし値はそこまで無傷で運ばれます。つまり「デコーダを通った」ことは、その URL を他の用途に使ってよい根拠にはなりません。

reloadable component のパラメータは公開入力です。[httpbind_reloadable_componet.ja.md](httpbind_reloadable_componet.ja.md) を参照してください。

### 文脈規則を持たない位置

まだ通常の属性テキストとして扱われているシンクが2つあります。

- `style` は `string` を受け付け、CSS escape ではなく HTML escape がかかります。かつてスクリプトを実行できた CSS のペイロードは現行ブラウザから消えているので、これは開いた穴というより hardening の残件です。
- `<iframe>` の `srcdoc` は HTML 文書そのものを持ちます。URL でもスクリプトでもなく、まだ固有の規則を持っていません。

## ここまでで守られないもの

以上の規則が縛るのは、ある位置で値が**何になれるか**です。その値がそこにあってよいかどうかについては何も言っていません。

フィルタを通った `href` は、許可されたスキームが連れて行く先ならどこでも指します。`ping` と `srcset` はパースできる行き先をすべて受け付けます。認可、所有権の確認、オープンリダイレクト対策は、引き続きアプリケーション側の仕事です。
