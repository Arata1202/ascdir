import Link from "next/link";
import { pageMetadata } from "@/src/lib/site";

export const metadata = pageMetadata({
  title: "ascdir and fastlane",
  description: "Understand where ascdir fits alongside fastlane in an App Store release workflow.",
  path: "/comparison/fastlane/",
});

const rows = [
  {
    area: "Primary focus",
    ascdir: "Reviewable App Store Connect state",
    fastlane: "End-to-end mobile release automation",
  },
  {
    area: "Configuration",
    ascdir: "YAML, Markdown, and managed asset directories",
    fastlane: "Ruby Fastfiles and action-specific metadata directories",
  },
  {
    area: "Preview",
    ascdir: "Read-only operation plans across supported workflows",
    fastlane: "HTML metadata preview in deliver; behavior varies by action",
  },
  {
    area: "Scope",
    ascdir: "Metadata, TestFlight distribution, submission, and release",
    fastlane: "Build, sign, test, upload, deploy, and more",
  },
  {
    area: "Best fit",
    ascdir: "Teams wanting explicit review of App Store Connect changes",
    fastlane: "Teams wanting a broad mobile automation toolbox",
  },
];

export default function FastlaneComparison() {
  return (
    <main className="contentPage shell" id="main-content">
      <header className="pageIntro">
        <p className="kicker">Comparison</p>
        <h1>Different tools for different release workflows.</h1>
        <p>
          ascdir is a focused App Store Connect CLI. fastlane is a broad automation ecosystem. They
          can be alternatives for some tasks or work together.
        </p>
      </header>
      <div
        className="comparison"
        role="region"
        aria-label="ascdir and fastlane comparison"
        tabIndex={0}
      >
        <table>
          <thead>
            <tr>
              <th>Area</th>
              <th>ascdir</th>
              <th>fastlane</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.area}>
                <th>{row.area}</th>
                <td>{row.ascdir}</td>
                <td>{row.fastlane}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="comparisonCards" aria-label="ascdir and fastlane comparison">
        {rows.map((row) => (
          <section key={row.area}>
            <h2>{row.area}</h2>
            <dl>
              <div>
                <dt>ascdir</dt>
                <dd>{row.ascdir}</dd>
              </div>
              <div>
                <dt>fastlane</dt>
                <dd>{row.fastlane}</dd>
              </div>
            </dl>
          </section>
        ))}
      </div>
      <p className="comparisonSources">
        Reviewed August 2026 against fastlane&apos;s official{" "}
        <a href="https://docs.fastlane.tools/actions/appstore/">deliver documentation</a> and{" "}
        <a href="https://docs.fastlane.tools/getting-started/ios/appstore-deployment/">
          App Store deployment guide
        </a>
        .
      </p>
      <section className="callout">
        <h2>Choose the tool that matches your workflow.</h2>
        <p>
          Use ascdir when reviewable App Store Connect changes and safe dry runs are the priority.
          Use fastlane when you need build, signing, testing, and deployment in one ecosystem. They
          can also work together: fastlane can build and upload the binary, then ascdir can manage
          metadata, TestFlight distribution, submission, and release.
        </p>
        <Link href="/docs/getting-started/">Get started with ascdir →</Link>
      </section>
    </main>
  );
}
