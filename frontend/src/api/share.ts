import { fetchURL, fetchJSON, removePrefix, createURL } from "./utils";

export async function list() {
  return fetchJSON<Share[]>("/api/shares");
}

export async function get(url: string) {
  url = removePrefix(url);
  return fetchJSON<Share[]>(`/api/share${url}`);
}

export async function remove(hash: string) {
  await fetchURL(`/api/share/${hash}`, {
    method: "DELETE",
  });
}

export async function create(
  url: string,
  password = "",
  expires = "",
  unit = "hours",
  pathPublic = false
) {
  url = removePrefix(url);
  url = `/api/share${url}`;
  if (expires !== "") {
    url += `?expires=${expires}&unit=${unit}`;
  }
  let body = "{}";
  if (password != "" || expires !== "" || unit !== "hours" || pathPublic) {
    body = JSON.stringify({
      password: password,
      expires: expires.toString(), // backend expects string not number
      unit: unit,
      pathPublic: pathPublic,
    });
  }
  return fetchJSON<Share>(url, {
    method: "POST",
    body: body,
  });
}

export function getShareURL(share: Share) {
  return createURL("share/" + share.hash, {});
}

export function getPathPublicURL(share: Share, inline = false) {
  if (!share.pathPublic) return "";
  const params = inline ? { inline: "true" } : {};
  return createURL("api/public/files" + share.path, params);
}
