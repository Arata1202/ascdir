import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "ascdir and fastlane",
  description: "Understand where ascdir fits alongside fastlane in an App Store release workflow.",
  alternates: { canonical: "/comparison/fastlane/" },
};

const rows = [
  ["Primary focus", "Reviewable App Store Connect state", "Broad mobile release automation"],
  ["Configuration", "Files designed for code review", "Ruby configuration and actions"],
  ["Preview", "Read-only plans before mutation", "Action-dependent"],
  ["Scope", "Metadata, TestFlight, submission, release", "Build, sign, test, deploy, and more"],
  [
    "Best fit",
    "Teams wanting explicit App Store changes",
    "Teams wanting an extensive automation toolbox",
  ],
];

export default function FastlaneComparison() {
  return (
    <main className="contentPage shell" id="main-content">
      <header className="pageIntro">
        <p className="kicker">Comparison</p>
        <h1>ascdir and fastlane solve different-sized problems.</h1>
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
            {rows.map(([area, ascdir, fastlane]) => (
              <tr key={area}>
                <th>{area}</th>
                <td>{ascdir}</td>
                <td>{fastlane}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <section className="callout">
        <h2>Choose the smallest tool that owns the workflow clearly.</h2>
        <p>
          Use ascdir when reviewable App Store state and safe dry runs are the priority. Keep
          fastlane where its wider build and signing ecosystem already serves you well.
        </p>
        <Link href="/docs/getting-started/">Get started with ascdir →</Link>
      </section>
    </main>
  );
}
