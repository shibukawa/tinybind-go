package main

const indexJavaScript = `
async function runStream(accept) {
  const out = document.getElementById('stream-out');
  out.textContent = '…';
  const msg = document.getElementById('msg').value || 'hello';
  const res = await fetch('/chat', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Accept': accept,
    },
    body: JSON.stringify({ message: msg }),
  });
  out.textContent = 'HTTP ' + res.status + '  Content-Type: ' + res.headers.get('content-type') + '\n\n';
  const reader = res.body.getReader();
  const dec = new TextDecoder();
  while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    out.textContent += dec.decode(value, { stream: true });
  }
}
document.getElementById('btn-sse').onclick = () => runStream('text/event-stream');
document.getElementById('btn-nd').onclick = () => runStream('application/x-ndjson');
document.getElementById('btn-ja').onclick = () => runStream('application/json');
`
