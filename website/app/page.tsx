import { JsonLd } from "@/src/components/json-ld";
import { absoluteUrl, site } from "@/src/lib/site";
import Image from "next/image";
import { CopyButton } from "@/src/components/copy-button";

const githubUrl = "https://github.com/Arata1202/ascdir";
const docsUrl = "/docs/getting-started/";
const installCommand = "brew install Arata1202/tap/ascdir";

const workflow = [
  {
    step: "01",
    title: "Bring it into your repository",
    body: "Initialize from an existing app and keep localized metadata and product-page assets in your repository.",
    command: "ascdir init --bundle-id com.example.app --platform IOS --version 1.2.0",
  },
  {
    step: "02",
    title: "Review the plan",
    body: "Validate locally, then inspect the exact App Store Connect changes before anything is applied.",
    command: "ascdir push --dry-run",
  },
  {
    step: "03",
    title: "Ship with intent",
    body: "Distribute an uploaded build through TestFlight, submit it for review, and release it after approval.",
    command: "ascdir app-store submit --confirm 1.2.0",
  },
];

const features = [
  {
    label: "Metadata",
    title: "A reviewable source of truth",
    body: "Keep product-page text, screenshots, previews, pricing, and availability in reviewable YAML, Markdown, and asset directories.",
  },
  {
    label: "TestFlight",
    title: "Repeatable distribution",
    body: "Resolve processed builds, attach them to existing internal or external groups, and request Beta App Review only when needed.",
  },
  {
    label: "App Store",
    title: "Release without guesswork",
    body: "Create a version, select a valid build, submit it for App Review, and publish approved manual releases from the terminal.",
  },
];

function ArrowIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 20 20" width="18" height="18">
      <path
        d="M4 10h11M11 6l4 4-4 4"
        fill="none"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.6"
      />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 20 20" width="18" height="18">
      <path
        d="m4.5 10 3.2 3.2 7.8-7.8"
        fill="none"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.8"
      />
    </svg>
  );
}

export default function Home() {
  const softwareJsonLd = {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    name: "ascdir",
    applicationCategory: "DeveloperApplication",
    operatingSystem: "macOS, Linux, Windows",
    description: site.description,
    url: absoluteUrl("/"),
    downloadUrl: `${site.github}/releases`,
    softwareHelp: absoluteUrl("/docs/"),
    license: `${site.github}/blob/main/LICENSE`,
    isAccessibleForFree: true,
  };

  return (
    <main id="main-content">
      <JsonLd data={softwareJsonLd} />
      <section className="hero shell" id="top">
        <div className="eyebrow">
          <span /> Open source App Store workflows
        </div>
        <h1>
          <span className="heroDesktop">Your App Store release,</span>
          <span className="heroDesktop heroMuted">reviewed before it ships.</span>
          <span className="heroMobile">
            Review your release.
            <br />
            Before it ships.
          </span>
        </h1>
        <p className="heroCopy">
          Manage metadata and product-page assets as files, then review TestFlight distribution and
          App Store release operations with safe dry runs.
        </p>
        <div className="heroActions">
          <a className="button buttonPrimary" href={docsUrl}>
            Get started
            <ArrowIcon />
          </a>
          <a className="button buttonSecondary" href={githubUrl}>
            View on GitHub
          </a>
        </div>
        <div className="install" aria-label="Homebrew install command">
          <span className="prompt">$</span>
          <code>{installCommand}</code>
          <span className="installHint">macOS · Linux</span>
          <CopyButton
            value={installCommand}
            label="Copy install command"
            variant="icon"
            className="installCopy"
          />
        </div>
      </section>

      <section className="demo shell" aria-labelledby="demo-title">
        <div className="sectionHeading demoHeading">
          <div>
            <p className="kicker">See the change before Apple does</p>
            <h2 id="demo-title">A release plan you can actually review.</h2>
          </div>
          <p>
            Human-readable plans for local changes, TestFlight distribution, submissions, and
            releases.
          </p>
        </div>
        <div className="terminalFrame">
          <div className="terminalBar">
            <div className="terminalDots">
              <span />
              <span />
              <span />
            </div>
            <span>ascdir — dry run</span>
          </div>
          <Image
            src="/ascdir-demo.gif"
            alt="ascdir previewing App Store Connect changes in a terminal"
            width={960}
            height={540}
            priority
            unoptimized
          />
        </div>
      </section>

      <section className="workflow shell" id="workflow" aria-labelledby="workflow-title">
        <div className="sectionHeading">
          <div>
            <p className="kicker">A focused workflow</p>
            <h2 id="workflow-title">From repository to release.</h2>
          </div>
          <p>
            ascdir starts after your app exists in App Store Connect and complements the build
            pipeline you already use.
          </p>
        </div>
        <div className="workflowGrid">
          {workflow.map((item) => (
            <article className="workflowCard" key={item.step}>
              <span className="step">{item.step}</span>
              <h3>{item.title}</h3>
              <p>{item.body}</p>
              <code>{item.command}</code>
            </article>
          ))}
        </div>
      </section>

      <section className="features shell" id="features" aria-labelledby="features-title">
        <div className="sectionHeading">
          <div>
            <p className="kicker">Built for recurring release work</p>
            <h2 id="features-title">Every release step stays explicit.</h2>
          </div>
        </div>
        <div className="featureList">
          {features.map((feature) => (
            <article className="feature" key={feature.label}>
              <span>{feature.label}</span>
              <h3>{feature.title}</h3>
              <p>{feature.body}</p>
            </article>
          ))}
        </div>
      </section>

      <section className="safety shell" aria-labelledby="safety-title">
        <div className="safetyIntro">
          <p className="kicker">Safe by design</p>
          <h2 id="safety-title">Automation that knows when to stop.</h2>
          <p>
            Apple does not provide transactions across every App Store Connect resource. ascdir
            stages complete plans, revalidates remote state, and refuses ambiguous operations.
          </p>
        </div>
        <ul className="safetyList">
          <li>
            <CheckIcon />
            <span>
              <strong>Dry run first</strong>Read-only plans before any remote change is applied.
            </span>
          </li>
          <li>
            <CheckIcon />
            <span>
              <strong>Explicit confirmation</strong>Bind execution to the configured app version.
            </span>
          </li>
          <li>
            <CheckIcon />
            <span>
              <strong>Idempotent retries</strong>Converge from current remote state without
              duplicate submissions.
            </span>
          </li>
          <li>
            <CheckIcon />
            <span>
              <strong>Short-lived auth</strong>Generate ES256 JWTs locally without uploading private
              keys.
            </span>
          </li>
        </ul>
      </section>

      <section className="cta shell">
        <div>
          <p className="kicker">Ready when your build is</p>
          <h2>Make the next release reviewable.</h2>
        </div>
        <div className="ctaActions">
          <a className="button buttonLight" href={docsUrl}>
            Read the quick start <ArrowIcon />
          </a>
        </div>
      </section>
    </main>
  );
}
