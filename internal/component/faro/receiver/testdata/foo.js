function throwError() {
  throw new Error('This is a thrown error');
}
function callUndefined() {
  // eslint-disable-next-line no-eval
  eval('test();');
}
function callConsole(method) {
  // eslint-disable-next-line no-console
  console[method](`This is a console ${method} message`);
}
function fetchError() {
  fetch('http://localhost:12345', {
      method: 'POST'
  });
}
function promiseReject() {
  new Promise((_accept, reject)=>{
      reject('This is a rejected promise');
  });
}
function fetchSuccess() {
  fetch('http://localhost:1234');
}
function sendCustomMetric() {
  window.grafanaJavaScriptAgent.api.pushMeasurement({
      type: 'custom',
      values: {
          my_custom_metric: Math.random()
      }
  });
}
window.addEventListener('load', ()=>{
  window.grafanaJavaScriptAgent.api.pushLog([
      'Manual event from Home'
  ]);
});

//# sourceMappingURL=foo.js.map
