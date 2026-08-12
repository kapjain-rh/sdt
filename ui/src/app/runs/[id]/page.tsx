import RunProgressClient from "./RunProgressClient";

export async function generateStaticParams() {
  return [{ id: "0" }];
}

export default function RunProgressPage() {
  return <RunProgressClient />;
}
