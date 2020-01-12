//window.audioCtx = new (window.AudioContext || window.webkitAudioContext)();

var aud = null;
var ws = null;
var ctrack = null;
var wsInterval = null;
class musicPlayer {
  constructor() {
    this.play = this.play.bind(this);
    this.playBtn = document.getElementById("play");
    this.playBtn.addEventListener("click", this.play);
    this.controlPanel = document.getElementById("control-panel");
    this.infoBar = document.getElementById("info");
    this.isPlaying = false;
  }

  play() {
    let controlPanelObj = this.controlPanel,
      infoBarObj = this.infoBar;
    Array.from(controlPanelObj.classList).find(function(element) {
      return element !== "active"
        ? controlPanelObj.classList.add("active")
        : controlPanelObj.classList.remove("active");
    });

    Array.from(infoBarObj.classList).find(function(element) {
      return element !== "active"
        ? infoBarObj.classList.add("active")
        : infoBarObj.classList.remove("active");
    });
    var aud = document.getElementById("audio-player");
    if (!this.isPlaying) {
      aud.src = `http://${window.location.host}/audio`;
      aud.muted = false;
      aud.play();
    } else {
      aud.muted = true;
    }

    this.isPlaying = !this.isPlaying;
  }
}

const newMusicplayer = new musicPlayer();

function enqueue() {
  q = document.getElementById("query").value.trim();
  if (ws === null) return;
  ws.send(JSON.stringify({ op: 3, query: q }));
}
function setTrack(track) {
  console.log(track);
  if (track === null) {
    return;
    let infoBox = document.getElementById("info");
    infoBox.getElementsByClassName("artist")[0].innerText = "";
    infoBox.getElementsByClassName("name")[0].innerText = "";
    let artworkBox = document.getElementsByClassName("album-art")[0];
    artworkBox.style.backgroundImage = ``;
  }
  ctrack = track;
  let infoBox = document.getElementById("info");
  infoBox.getElementsByClassName("artist")[0].innerText = ctrack.artist.name;
  infoBox.getElementsByClassName("name")[0].innerText = ctrack.title;
  let artworkBox = document.getElementsByClassName("album-art")[0];
  artworkBox.style.backgroundImage = `url(${ctrack.album.cover})`;
}
function setListeners(count) {
  let infoBox = document.getElementById("info");
  infoBox.getElementsByClassName("listeners")[0].innerText = `🎧: ${count}`;
}
function initWebSocket() {
  ws = new WebSocket(`ws://${window.location.host}/status`);
  ws.onerror = err => {
    console.log(err);
  };
  ws.onopen = event => {
    console.log("[WS] opened");
    ws.send(JSON.stringify({ op: 1 }));
    wsInterval = setInterval(() => {
      ws.send(JSON.stringify({ op: 8 }));
    }, 30000);
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
      case 3:
        let subBox = document.getElementById("sub");
        let artistBox = subBox.getElementsByClassName("artist")[0];
        let titleBox = subBox.getElementsByClassName("name")[0];
        artistBox.innerText = "";
        titleBox.innerText = "";
        if (!msg.success) {
          titleBox.innerText = msg.reason;
        } else {
          titleBox.innerText = msg.track.title;
          artistBox.innerText = msg.track.artist.name;
        }
        Array.from(subBox.classList).find(function(element) {
          return element !== "active"
            ? subBox.classList.add("active")
            : subBox.classList.remove("active");
        });
        setTimeout(() => {
          Array.from(subBox.classList).find(function(element) {
            return element !== "active"
              ? subBox.classList.add("active")
              : subBox.classList.remove("active");
          });
        }, 5000);
        document.getElementById("query").value = "";
        break;
      case 5:
        setListeners(msg.listeners);
        break;
      default:
        break;
    }
  };
}
const node = document.getElementsByClassName("query-track")[0];
node.addEventListener("keydown", function(event) {
  if (event.key === "Enter") {
    event.preventDefault();
    enqueue();
  }
});
window.onload = function() {
  this.initWebSocket();
};
