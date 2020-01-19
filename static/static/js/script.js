//window.audioCtx = new (window.AudioContext || window.webkitAudioContext)();

var aud = null;
var ws = null;
var ctrack = null;
var wsInterval = null;
class musicPlayer {
  constructor() {
    this.play = this.play.bind(this);
    this.skip = this.skip.bind(this);
    this.skipBtn = document.getElementById("skip");
    this.skipBtn.addEventListener("click", this.skip);
    this.playBtn = document.getElementById("play");
    this.playBtn.addEventListener("click", this.play);
    this.controlPanel = document.getElementById("control-panel");
    this.isPlaying = false;
  }
  skip() {
    ws.send(JSON.stringify({ op: 4 }));
  }
  play() {
    var aud = document.getElementById("audio-player");
    if (!this.isPlaying) {
      this.playBtn.classList.add("playing")
      aud.src = `http://${window.location.host}/audio`;
      aud.muted = false;
      aud.play();
    } else {
      this.playBtn.classList.remove("playing")
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
  }
  ctrack = track;
  document.getElementById("artist").innerText = ctrack.artist.name;
  document.getElementById("name").innerText = ctrack.title;
  //let artworkBox = document.getElementsByClassName("album-art")[0];
  //artworkBox.style.backgroundImage = `url(${ctrack.album.cover})`;
}
function setListeners(count) {
  document.getElementById("listeners").innerText = `Listeners: ${count}`;
}
function initWebSocket() {
  if (window.location.protocol=="http:")
  {
    ws = new WebSocket(`ws://${window.location.host}/status`);
  }
  else{
    ws = new WebSocket(`wss://${window.location.host}/status`);
  }
  ws.onerror = err => {
    console.log(err);
    ws.close()
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
    clearInterval(wsInterval)
    setTimeout(initWebSocket,1000)
  };
  ws.onmessage = event => {
    console.log(event.data);
    var msg = JSON.parse(event.data);
    switch (msg.op) {
      case 1:
        setTrack(msg.track);
        break;
      case 3:
        var subBox = document.getElementById("sub");
        var artistBox = subBox.getElementsByClassName("artist")[0];
        var titleBox = subBox.getElementsByClassName("name")[0];
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
      case 4:
        var subBox = document.getElementById("sub");
        var artistBox = subBox.getElementsByClassName("artist")[0];
        var titleBox = subBox.getElementsByClassName("name")[0];
        artistBox.innerText = "";
        titleBox.innerText = "";
        if (!msg.success) {
          titleBox.innerText = msg.reason;
        } else {
          titleBox.innerText = "Skipped!";
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
        }, 2000);
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
