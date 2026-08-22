import Link from "next/link";

export function Brand() {
  return (
    <Link className="brand" href="/" aria-label="ascdir home">
      <span className="brandMark" aria-hidden="true">
        asc
      </span>
      <span>ascdir</span>
    </Link>
  );
}
