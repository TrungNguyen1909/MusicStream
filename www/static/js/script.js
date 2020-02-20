var ws = null;
var ctrack = null;
var wsInterval = null;
var lyricsInterval = null;
var subBoxTimeout = null;
var delta = 0;
var isSkipped = false;
const opSetClientsTrack = 1;
const opAllClientsSkip = 2;
const opClientRequestTrack = 3;
const opClientRequestSkip = 4;
const opSetClientsListeners = 5;
const opTrackEnqueued = 6;
const opClientRequestQueue = 7;
const opWebSocketKeepAlive = 8;
const opClientRemoveTrack = 9;
class musicPlayer {
  constructor() {
    this.play = this.play.bind(this);
    //this.pause = this.pause.bind(this);
    this.skip = this.skip.bind(this);
    this.skipBtn = document.getElementById("skip");
    this.skipBtn.addEventListener("click", this.skip);
    this.playBtn = document.getElementById("play");
    this.playBtn.addEventListener("click", this.play);
    //this.pauseBtn = document.getElementById("pause");
    //this.pauseBtn.addEventListener("click", this.pause);
    this.controlPanel = document.getElementById("control-panel");
    this.isPlaying = false;
  }
  skip() {
    ws.send(JSON.stringify({ op: opClientRequestSkip }));
    this.skipBtn.disabled = true;
    setTimeout(() => {
      this.skipBtn.disabled = false;
    }, 1000);
  }
  play() {
    if (!this.isPlaying) {
      this.controlPanel.classList.add("playing");
      window.player.muted = false;
      window.player.play();
      this.isPlaying = 1;
    } else {
      this.controlPanel.classList.remove("playing");
      window.player.muted = true;
      this.isPlaying = 0;
    }
  }
}

const newMusicplayer = new musicPlayer();
var mode = 1;
var dzSel = document.getElementById("deezer-sel");
var csnSel = document.getElementById("csn-sel");

function applySelector() {
  if (mode == 1) {
    csnSel.classList.remove("active");
    dzSel.classList.add("active");
  } else {
    csnSel.classList.add("active");
    dzSel.classList.remove("active");
  }
  localStorage.setItem("src-selector", mode);
}

function initSelector() {
  let selector = localStorage.getItem("src-selector");
  if (!selector) {
    mode = 1;
    applySelector();
    return;
  } else {
    mode = +selector;
    applySelector();
  }
}

dzSel.addEventListener("click", () => {
  mode = 1;
  applySelector();
});

csnSel.addEventListener("click", () => {
  mode = 2;
  applySelector();
});

function enqueue() {
  q = document.getElementById("query").value.trim();
  if (!ws) return;
  var subBox = document.getElementById("sub");
  var artistBox = subBox.getElementsByClassName("artist")[0];
  var titleBox = subBox.getElementsByClassName("name")[0];
  artistBox.innerText = `Query: ${q}`;
  titleBox.innerText = "Requesting song...";
  clearTimeout(subBoxTimeout);
  showSubBox();
  subBoxTimeout = setTimeout(hideSubBox, 2000);
  ws.send(
    JSON.stringify({ op: opClientRequestTrack, query: q, selector: mode })
  );
}
function setTrack(track) {
  console.log(track);
  if (!track) {
    return;
  }
  ctrack = track;
  artistBox = document.getElementById("artist");
  titleBox = document.getElementById("name");
  titleBox.classList.remove("marquee2");
  artistBox.classList.remove("marquee2");
  titleBox.style.setProperty("--indent-percent", "0%");
  artistBox.style.setProperty("--indent-percent", "0%");
  titleBox.style.textIndent = "0%";
  artistBox.style.textIndent = "0%";
  artistBox.innerText = ctrack.artists;
  titleBox.innerText = ctrack.title;
  if (isElementOverflowing(titleBox)) {
    titleBox.style.setProperty(
      "--indent-percent",
      -(titleBox.scrollWidth / titleBox.offsetWidth) * 100 + 100 + "%"
    );
    titleBox.classList.add("marquee2");
  }
  if (isElementOverflowing(artistBox)) {
    artistBox.style.setProperty(
      "--indent-percent",
      -(artistBox.scrollWidth / artistBox.offsetWidth) * 100 + 100 + "%"
    );
    artistBox.classList.add("marquee2");
  }
  // window.player.src = `/audio`;
  lyricsControl();
  //let artworkBox = document.getElementsByClassName("album-art")[0];
  //artworkBox.style.backgroundImage = `url(${ctrack.album.cover})`;
}
function setListeners(count) {
  document.getElementById("listeners").innerText = `Listeners: ${count}`;
}
function showSubBox() {
  subBox = document.getElementById("sub");
  subBox.classList.add("active");
}
function hideSubBox() {
  subBox = document.getElementById("sub");
  subBox.classList.remove("active");
}
function toggleSubBox() {
  subBox = document.getElementById("sub");
  Array.from(subBox.classList).find(function(element) {
    return element !== "active"
      ? subBox.classList.add("active")
      : subBox.classList.remove("active");
  });
}
function showLyricsBox() {
  lyricsBox = document.getElementById("lyrics");
  lyricsBox.classList.add("active");
}
function hideLyricsBox() {
  lyricsBox = document.getElementById("lyrics");
  lyricsBox.classList.remove("active");
}
function toggleLyricsBox() {
  lyricsBox = document.getElementById("lyrics");
  Array.from(lyricsBox.classList).find(function(element) {
    return element !== "active"
      ? lyricsBox.classList.add("active")
      : lyricsBox.classList.remove("active");
  });
}
function initWebSocket() {
  if (window.location.protocol == "http:") {
    ws = new WebSocket(`ws://${window.location.host}/status`);
  } else {
    ws = new WebSocket(`wss://${window.location.host}/status`);
  }
  ws.onerror = err => {
    console.log(err);
    ws.close();
  };
  ws.onopen = event => {
    console.log("[WS] opened");
    ws.send(JSON.stringify({ op: opSetClientsTrack }));
    ws.send(JSON.stringify({ op: opClientRequestQueue }));
    wsInterval = setInterval(() => {
      ws.send(JSON.stringify({ op: opWebSocketKeepAlive }));
    }, 30000);
  };
  ws.onclose = event => {
    console.log("[WS] closed");
    clearInterval(wsInterval);
    setTimeout(initWebSocket, 1000);
  };
  ws.onmessage = event => {
    var msg = JSON.parse(event.data);
    switch (msg.op) {
      case opSetClientsTrack:
        if (document.getElementById("queue").childElementCount > 0) {
          if (
            document.getElementById("queue").firstChild.playId ==
            msg.track.playId
          )
            document
              .getElementById("queue")
              .removeChild(document.getElementById("queue").firstChild);
        }
        delta = msg.pos / 48000.0;
        diff = delta - player.currentTime;
        if (
          isSkipped ||
          !ctrack ||
          (ctrack.source == 3 && msg.track.source != 3)
        ) {
          player.src = `/audio`;
        } else if (Math.abs(diff) > 8) {
          if (msg.track.source == 3) {
            setTimeout(() => {
              player.src = `/audio`;
            }, (diff - 3.168) * 1000);
          } else {
            player.src = `/audio`;
          }
        }
        console.log(`Audio diff: ${diff}`);
        isSkipped = false;
        setTrack(msg.track);
        setListeners(msg.listeners);
        break;
      case opAllClientsSkip:
        isSkipped = true;
        break;
      case opClientRequestTrack:
        var subBox = document.getElementById("sub");
        var artistBox = subBox.getElementsByClassName("artist")[0];
        var titleBox = subBox.getElementsByClassName("name")[0];
        artistBox.innerText = "";
        titleBox.innerText = "";
        if (!msg.success) {
          titleBox.innerText = msg.reason;
        } else {
          titleBox.innerText = msg.track.title;
          artistBox.innerText = msg.track.artists;
        }
        clearTimeout(subBoxTimeout);
        showSubBox();
        subBoxTimeout = setTimeout(hideSubBox, 3000);
        document.getElementById("query").value = "";
        break;
      case opClientRequestSkip:
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
        clearTimeout(subBoxTimeout);
        showSubBox();
        subBoxTimeout = setTimeout(hideSubBox, 2000);
        break;
      case opSetClientsListeners:
        setListeners(msg.listeners);
        break;
      case opTrackEnqueued:
        {
          let ele = document.createElement("div");
          ele.className = "element";
          ele.playId = msg.track.playId;
          ele.addEventListener("contextmenu", this.removeTrack);
          let titleBox = document.createElement("div");
          titleBox.className = "title";
          titleBox.innerText = msg.track.title;
          titleBox.playId = msg.track.playId;
          ele.appendChild(titleBox);
          let artistBox = document.createElement("div");
          artistBox.className = "artist";
          artistBox.innerText = msg.track.artists;
          artistBox.playId = msg.track.playId;
          ele.appendChild(artistBox);
          document.getElementById("queue").appendChild(ele);
        }
        break;
      case opClientRequestQueue:
        while (document.getElementById("queue").firstChild) {
          document
            .getElementById("queue")
            .removeChild(document.getElementById("queue").firstChild);
        }
        msg.queue.forEach(track => {
          let ele = document.createElement("div");
          ele.className = "element";
          ele.playId = track.playId;
          ele.addEventListener("contextmenu", this.removeTrack);
          let titleBox = document.createElement("div");
          titleBox.className = "title";
          titleBox.innerText = track.title;
          titleBox.playId = track.playId;
          ele.appendChild(titleBox);
          let artistBox = document.createElement("div");
          artistBox.className = "artist";
          artistBox.innerText = track.artists;
          artistBox.playId = track.playId;
          ele.appendChild(artistBox);
          document.getElementById("queue").appendChild(ele);
        });
        break;
      case opClientRemoveTrack:
        if (!msg.success) {
          for (let child of document.getElementById("queue").children) {
            if (child.playId == msg.track.playId) {
              child.disabled = false;
              break;
            }
          }
          break;
        }
        var subBox = document.getElementById("sub");
        var artistBox = subBox.getElementsByClassName("artist")[0];
        var titleBox = subBox.getElementsByClassName("name")[0];
        artistBox.innerText = "";
        titleBox.innerText = "";
        titleBox.innerText = `Removing`;
        artistBox.innerText = `${msg.track.title} - ${msg.track.artist}`;
        for (let child of document.getElementById("queue").children) {
          if (child.playId == msg.track.playId) {
            child.remove();
            break;
          }
        }
        clearTimeout(subBoxTimeout);
        showSubBox();
        subBoxTimeout = setTimeout(hideSubBox, 3000);
        break;
      default:
        break;
    }
  };
}
function removeTrack(event) {
  console.log(event);
  event.preventDefault();
  console.log(event.currentTarget.playId);
  ws.send(
    JSON.stringify({ op: opClientRemoveTrack, query: event.target.playId })
  );
  var subBox = document.getElementById("sub");
  var artistBox = subBox.getElementsByClassName("artist")[0];
  var titleBox = subBox.getElementsByClassName("name")[0];
  artistBox.innerText = "";
  titleBox.innerText = "";
  titleBox.innerText = `Removing`;
  artistBox.innerText = `${
    event.currentTarget.getElementsByClassName("title")[0].innerText
  } - ${event.currentTarget.getElementsByClassName("artist")[0].innerText}`;
  event.currentTarget.disabled = true;
  clearTimeout(subBoxTimeout);
  showSubBox();
  subBoxTimeout = setTimeout(hideSubBox, 3000);
}
var enterPressed = false;
const search = document.getElementById("query");
search.addEventListener("keydown", function(event) {
  if (event.key === "Enter" && !enterPressed) {
    event.preventDefault();
    enterPressed = true;
    setTimeout(() => {
      enterPressed = false;
    }, 1000);
    enqueue();
  }
});
window.onload = function() {
  this.initSelector();
  this.player = document.getElementById("audio-player");
  this.player.onload = () => {
    this.fetch("/listeners")
      .then(response => response.json())
      .then(msg => this.setListeners(msg.listeners));
  };
  this.player.onerror = () => {
    this.player.src = `/audio`;
  };
  this.initWebSocket();
};

function lyricsControl() {
  clearInterval(lyricsInterval);
  hideLyricsBox();
  var player = document.getElementById("audio-player");
  var lyricsBox = document.getElementById("lyrics");
  let originalBox = lyricsBox.getElementsByClassName("original")[0];
  let translatedBox = lyricsBox.getElementsByClassName("translated")[0];
  originalBox.innerText = "";
  translatedBox.innerText = "";
  originalBox.style.transitionDuration = "0s";
  translatedBox.style.transitionDuration = "0s";
  originalBox.style.transitionDelay = "0s";
  translatedBox.style.transitionDelay = "0s";
  originalBox.style.textIndent = "0%";
  translatedBox.style.textIndent = "0%";
  if (!ctrack.lyrics || !ctrack.lyrics.lrc) {
    return;
  }
  showLyricsBox();
  let idx = 0;
  let lyricsChanged = false;
  lyricsInterval = setInterval(() => {
    {
      while (
        ctrack.lyrics.lrc[idx].time.total + window.delta <=
        player.currentTime
      ) {
        idx++;
        lyricsChanged = true;
        if (idx >= ctrack.lyrics.lrc.length) {
          hideLyricsBox();
          clearInterval(lyricsInterval);
          break;
        }
      }
      if (!lyricsChanged) return;
      lyricsChanged = false;
      originalBox.innerText = "";
      translatedBox.innerText = "";
      originalBox.style.transitionDuration = "0s";
      translatedBox.style.transitionDuration = "0s";
      originalBox.style.transitionDelay = "0s";
      translatedBox.style.transitionDelay = "0s";
      originalBox.style.textIndent = "0%";
      translatedBox.style.textIndent = "0%";
      originalBox.innerText = ctrack.lyrics.lrc[idx - 1].text;
      translatedBox.innerText = ctrack.lyrics.lrc[idx - 1].translated;
      let delta =
        idx < ctrack.lyrics.lrc.length
          ? ctrack.lyrics.lrc[idx].time.total -
            ctrack.lyrics.lrc[idx - 1].time.total
          : 10;
      if (isElementOverflowing(originalBox) && idx < ctrack.lyrics.lrc.length) {
        originalBox.style.transitionDuration = 2 * delta + "s";

        originalBox.style.transitionDelay = "1s";
        originalBox.style.textIndent =
          -(originalBox.scrollWidth / originalBox.offsetWidth) * 100 + "%";
      }
      if (
        isElementOverflowing(translatedBox) &&
        idx < ctrack.lyrics.lrc.length
      ) {
        translatedBox.style.transitionDuration = 2 * delta + "s";

        translatedBox.style.transitionDelay = "1s";
        translatedBox.style.textIndent =
          -(translatedBox.scrollWidth / translatedBox.offsetWidth) * 100 + "%";
      }
    }
  }, 100);
}

function isElementOverflowing(element) {
  var overflowX = element.offsetWidth < element.scrollWidth;

  return overflowX;
}
