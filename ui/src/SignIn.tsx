/**
 * The signed-out state. Sign-in is a full-page server-side redirect: no Google
 * script is loaded here, which is why the page needs no accounts.google.com
 * allowance.
 */
export default function SignIn() {
  return (
    <main className="signin">
      <h2>Sign in required.</h2>
      <p>Your snippets are private to your Google account.</p>
      <a href="/api/auth/google">Continue with Google</a>
    </main>
  )
}
