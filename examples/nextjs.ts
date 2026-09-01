// Server-side or client-side call. No credential is required.
// Keep the opaque token in your application/database so you know what it represents.

export async function count(token: string) {
  await fetch(`https://counter.example.com/x/${token}`, {
    method: 'POST',
    cache: 'no-store',
    keepalive: true,
  }).catch(() => {
    // Counter failures should normally never break the main application flow.
  })
}

// GET is also supported when needed:
export function countWithGet(token: string) {
  return fetch(`https://counter.example.com/x/${token}`, {
    method: 'GET',
    cache: 'no-store',
    keepalive: true,
  }).catch(() => undefined)
}
