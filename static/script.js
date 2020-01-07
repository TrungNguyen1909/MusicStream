window.audioCtx = new (window.AudioContext || window.webkitAudioContext)();
var ctrack = null;
var prevLine = "";
var ws = null;
var audioWS = null;
var lyricsInterval = null;
var ping = 0;
var tmp = new Uint8Array();
var aud = null;
function enqueue() {
  q = document.getElementById("query").value.trim();
  if (ws === null) return;
  ws.send(JSON.stringify({ op: 3, query: q }));
}
window.onload = () => {
  function setTrack(track) {
    console.log(track);
    if (track === null) return;
    ctrack = track;

    document.getElementById("title").innerText = track["title"];
    document.getElementById("artist").innerText = track["artist"]["name"];
    if (ctrack.lyrics) {
      ctrack.lyrics.lrc.reverse();
    }
  }
  function setLyricsFromWS(lyrics) {
    document.getElementById("lyrics").innerText = lyrics;
  }
  ws = new WebSocket(`ws://${window.location.host}/status`);
  ws.onerror = err => {
    console.log(err);
  };
  ws.onopen = event => {
    console.log("[WS] opened");
    ws.send(JSON.stringify({ op: 1 }));
    ws.send(JSON.stringify({ op: 2 }));
  };
  ws.onclose = event => {
    console.log("[WS] closed");
  };
  ws.onmessage = event => {
    console.log(event.data);
    var msg = JSON.parse(event.data);
    switch (msg.op) {
      case 1:
        setTrack(msg.track);
        break;
      case 2:
        setTimeout(setLyricsFromWS, ping, msg.lyrics);
        break;
      case 3:
        let result = document.getElementById("results");
        if (!msg.success) {
          var node = document.createElement("a");
          node.textContent = msg.reason;
          result.appendChild(node);
        } else {
          var div = document.createElement('div')
          var node = document.createElement("a");
          node.textContent = `Added track ${msg.track.title} - ${msg.track.artist.name}`;
          div.appendChild(node);
          result.appendChild(div)
        }
        break;
      default:
        break;
    }
  };
  
  document.querySelector("button").addEventListener("click", function() {
    window.audioCtx.resume();
    //setTimeout(play, 0);
    var aud = document.getElementById("audio-player");
    aud.src = `http://${window.location.host}/audio`;

    aud.play();
  });
};
