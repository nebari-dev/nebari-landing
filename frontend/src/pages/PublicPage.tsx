import type { ReactNode } from "react";
import { useState } from "react";

import logoUrlDark from "../assets/nebari-logo_dark.svg";
import logoUrlLight from "../assets/nebari-logo_light.svg";

import { Moon, Sun, ExternalLink } from "lucide-react";

import "./PublicPage.scss";

export default function PublicPage(): ReactNode {
    const [isDarkMode, setIsDarkMode] = useState(
        () => window.matchMedia("(prefers-color-scheme: dark)").matches,
    );

    const logoSrc = isDarkMode ? logoUrlDark : logoUrlLight;

    return (
        <div className={`public-shell ${isDarkMode ? "public-shell--dark" : "public-shell--light"}`}>
            {/* ── Header ──────────────────────────────────────────────────── */}
            <header className="public-header">
                <a href="/" className="public-header__brand">
                    <img src={logoSrc} className="public-header__logo" alt="Nebari" />
                </a>

                <nav className="public-header__nav">
                    <a className="public-header__link" href="/docs">
                        Docs <ExternalLink size={13} className="public-header__linkIcon" />
                    </a>
                    <a
                        className="public-header__link"
                        href="https://github.com/nebari-dev"
                        target="_blank"
                        rel="noreferrer"
                    >
                        GitHub <ExternalLink size={13} className="public-header__linkIcon" />
                    </a>
                </nav>

                <div className="public-header__actions">
                    <button
                        type="button"
                        className="public-header__iconBtn"
                        onClick={() => setIsDarkMode((v) => !v)}
                        title={isDarkMode ? "Switch to light mode" : "Switch to dark mode"}
                    >
                        {isDarkMode
                            ? <Sun size={16} />
                            : <Moon size={16} />}
                    </button>

                    <a
                        href="/oauth2/start?rd=/"
                        className="public-header__signInBtn"
                    >
                        Sign in
                    </a>
                </div>
            </header>

            {/* ── Hero ────────────────────────────────────────────────────── */}
            <main className="public-main">
                <section className="public-hero">
                    <h1 className="public-hero__title">
                        Welcome to&nbsp;<span className="public-hero__brand">Nebari</span>
                    </h1>
                    <p className="public-hero__subtitle">
                        A managed open-source data science platform for teams.
                        <br />
                        Sign in to access your services, notebooks, and more.
                    </p>

                    <div className="public-hero__ctas">
                        <a href="/oauth2/start?rd=/" className="public-cta public-cta--primary">
                            Sign in to get started
                        </a>
                        <a
                            href="https://nebari.dev"
                            target="_blank"
                            rel="noreferrer"
                            className="public-cta public-cta--secondary"
                        >
                            Learn more <ExternalLink size={14} />
                        </a>
                    </div>
                </section>
            </main>
        </div>
    );
}
