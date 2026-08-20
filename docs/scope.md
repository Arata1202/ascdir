# Product scope

ascdir focuses on recurring App Store metadata and release work that benefits
from reviewable files, dry runs, and repeatable automation. It complements
Xcode and App Store Connect rather than replacing either product.

## Core workflows

These workflows are part of ascdir's primary user experience:

- Pull, validate, diff, and push common localized product-page metadata
- Manage screenshots and App Preview videos as ordered local assets
- Inspect release and TestFlight state in human-readable or JSON form
- Create an App Store version, select an uploaded build, and submit it for review
- Request publication of an approved manual-release version
- Attach an uploaded build to existing TestFlight groups and request Beta App
  Review for external testing when required

Every mutating release command provides a read-only plan first, revalidates
remote state before execution, and requires the configured version as an
explicit confirmation token.

## Advanced opt-in workflows

The following supported resources are intentionally deeper in the
documentation because many individual apps configure them infrequently:

- [Age rating declarations](age-rating.md)
- [Accessibility Nutrition Labels](accessibility.md)
- [Custom license agreements](license-agreement.md)
- [Territory availability and preorders](availability.md)
- [Pricing schedules](pricing.md)
- [Screenshots](screenshots.md) and [App Previews](app-previews.md)

They remain first-class, tested features. Keeping them out of the shortest path
does not make them deprecated.

## Deliberate boundaries

ascdir does not manage:

- Xcode projects, archives, code signing, certificates, or provisioning profiles
- Build upload or processing
- Creation and membership of TestFlight groups or testers
- App Store Connect users, roles, agreements, tax, or banking information
- In-app purchases, subscriptions, Game Center, analytics, sales reports,
  customer reviews, or phased feature configuration
- One-off or specialized App Store Connect resources that are not represented
  in the documented schema

These boundaries keep confirmed operations understandable and avoid turning a
metadata and release tool into a second general-purpose App Store Connect UI.
New advanced resources should be added only when their API lifecycle,
validation rules, dry-run representation, and safe retry behavior are all
well-defined.
