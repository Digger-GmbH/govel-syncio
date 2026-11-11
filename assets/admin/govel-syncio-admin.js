import { mount } from "./connections.js";

export function mountSyncioAdmin(root, opts) {
  return mount(root, opts);
}

if (typeof window !== "undefined") {
  window.GovelSyncioAdmin = { mount: mountSyncioAdmin };
}
