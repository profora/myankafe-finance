import Link from "next/link";

export default function DevLogin(){
  return <div className="login-card">
    <div className="brand login-brand">MyanKafe <span>Finance</span></div>
    <h1>Development login retired</h1>
    <p className="muted">The finance app now uses the same username/password session model intended for production.</p>
    <Link className="button" href="/login">Go to sign in</Link>
  </div>;
}
