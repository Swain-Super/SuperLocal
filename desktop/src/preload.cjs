const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("launcher", {
  getState: () => ipcRenderer.invoke("launcher:getState"),
  start: (port) => ipcRenderer.invoke("launcher:start", port),
  stop: () => ipcRenderer.invoke("launcher:stop"),
  setPort: (port) => ipcRenderer.invoke("launcher:setPort", port),
  openChat: () => ipcRenderer.invoke("launcher:openChat"),
  openAdmin: () => ipcRenderer.invoke("launcher:openAdmin"),
  onState: (callback) => {
    const listener = (_event, next) => callback(next);
    ipcRenderer.on("launcher:state", listener);
    return () => ipcRenderer.removeListener("launcher:state", listener);
  },
});
