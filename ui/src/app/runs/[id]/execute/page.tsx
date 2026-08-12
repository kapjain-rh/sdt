import ExecuteClient from "./ExecuteClient";

export async function generateStaticParams() {
  return [{ id: "0" }];
}

export default function ExecutePage() {
  return <ExecuteClient />;
}
