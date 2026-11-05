export function beginQuit(alreadyQuitting, status) {
  if (alreadyQuitting) {
    return { quitting: true, hold: false };
  }
  return {
    quitting: true,
    hold: status === "starting" || status === "running",
  };
}
