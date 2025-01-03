---
theme: [dark, dashboard]
toc: false
---

```js
import {twoDayPlanetAttack, planetTableRows, getDefender, getLiberation, renderDefenses, renderAHTags, getMajorOrderDetails} from "./components/planet_history.js";
```

<style>

.hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  font-family: var(--sans-serif);
  margin: 1rem 0 2rem;
  text-wrap: balance;
  text-align: center;
}

.hero h1 {
  margin: 2rem 0;
  max-width: none;
  font-size: 14vw;
  font-weight: 900;
  line-height: 1;
  background: linear-gradient(30deg, var(--theme-foreground-focus), currentColor);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.hero h2 {
  margin: 0;
  max-width: 34em;
  font-size: 20px;
  font-style: initial;
  font-weight: 500;
  line-height: 1.5;
  color: var(--theme-foreground-muted);
}

@media (min-width: 640px) {
  .hero h1 {
    font-size: 90px;
  }
}

.card h2 {
  font-size: 32px;
  text-align:justify;
}

#map-container {
  position:relative;
}

#map {
  position: absolute;
  pointer-events: none;
  object-fit:cover;
  width: calc(100% - 2rem)
}

#map img {
  margin-top:1rem;
}

.center {
  width: 100%;
  text-align: center;
}

.center p {
  width: 100%;
  margin-inline: auto;
}

.center strong {
  font-size: calc(1.25rem + 0.25vw);
}

.center table{
  margin-inline: auto;  
}

.center td {
  padding: inherit auto;
  text-align: center;
}

.center th {
  padding: inherit auto;
  text-align: center;
}

.text-yellow {
  color: yellow;
}

.font-bold {
  font-weight: bold;
}

</style>

```js
const loadedAt = Date.now();
const eff_now = (async function*(){
  for(;;){
    yield Date.now();
    await new Promise(resolve => setTimeout(resolve, 1000));
  }
})();
```

```js
const lang = view(Inputs.select(["en-US", "de-DE", "es-ES", "ru-RU", "fr-FR", "it-IT", "pl-PL", "zh-Hans", "zh-Hant"], {value: "en", label: "Language", width: '7em'}));
const status = FileAttachment('./data/current_status.json').json().catch(() => window.location.reload());
const agg = FileAttachment('./data/aggregates.json').json();
const focus = FileAttachment('./data/recent_attacks.json').json();
const legendArrowURL = FileAttachment("./data/legend_arrow.svg").url();
```

```js
setTimeout(() => document.location.reload(), 10*60*1000);
const defenses = status.events.filter(e => e.type == 1);
const lastEntryTime = new Date(agg[agg.length-1].timestamp);
const lastEntryWarTime = new Date(status.war.now);
const GAME_EPOCH = new Date(status.war.start_time);
```

```js
const timeSinceLastEntry = new Date(eff_now - lastEntryTime);
const timeSinceGameEpoch = timeSinceLastEntry.getTime() + lastEntryWarTime.getTime();
```

<div class="hero">
  <h1>Helldivers Dashboard </h1>
  <h2>Welcome Helldiver! It is Day ${Math.floor(timeSinceGameEpoch / (1000 * 60 * 60 * 24)).toFixed()} of The Second Galactic War.</h2>
</div> 


<div class="warning" label="Watch out, Helldiver">
This page will automatically refresh every 10 minutes, the data is collected approximately every 10 minutes. This helps keep the servers stable and makes this website 100% ad and tracker free.
<br>
This page was last refreshed ${Math.floor((eff_now - loadedAt)/(60*1000)).toFixed()}m ${((eff_now - loadedAt)/1000).toFixed()%60}s ago.
</div>

```js
function getColor(owner) {
  switch(owner){
    case 'Terminids':
      return '#EF8E20';
      case 'Automaton':
        return '#EF2020';
      case 'Humans':
        return '#79E0FF';
      case 'Illuminate':
        return '#9634FF';
  }
}
function factionLegend(factions, {r = 5, strokeWidth = 2.5, width=640} = {}) {
  const frameAnchor = 'top-right';
  const factionMarks = factions.map((v,i) => {
      return [
        Plot.dot([v], {
          r: r,
          strokeWidth: strokeWidth,
          fill: getColor(v),
          stroke: getColor(v),
          frameAnchor,
          dx: (-6*16) - (r+strokeWidth),
          dy: 3*i*r,
        }),
        Plot.text([v], {
          text: [v],
          dx: -4*16,
          dy: 3*i*r-6,
          frameAnchor,
          textAnchor: 'middle',
        }),
      ]
    });
    let arrowMarks = [
      Plot.image(['Attack'],{ 
        frameAnchor: frameAnchor,
        r: r*1.5,
        dx: -6*16 - r,
        dy: 3*(factions.length)*r-2,
        src: legendArrowURL,
    }), Plot.text(['Attack'],{
      frameAnchor,
      text: ['Attack'],
      textAnchor: 'middle',
      dx: -4*16,
      dy: 3*(factions.length)*r-6
    })];

  return Plot.marks(factionMarks.concat(arrowMarks));
}
```

```js
const active = status.active.map(p => p.planet.index);
active.push(0);
```
<div class="grid grid-cols-2">
  <div class="card">
    <div>
    ${
      Inputs.table(status.dispatches, {
        header: {message: "Dispatch History"}, 
        columns:['message'],
        format: { message: x => htl.html`<span style="white-space:normal">${renderAHTags(x[lang])}`},
        layout: 'auto',
        }
      )
    }</div>
    <hr>
    <div class="center">
    ${ status.assignments[0] != undefined ? 
        htl.html`<strong>${renderAHTags(status.assignments[0].title[lang])}</strong>
        <br>
        ${renderAHTags(status.assignments[0].briefing[lang])}
        <br>
        ${renderAHTags(status.assignments[0].description[lang])}<br>
        Status: ${getMajorOrderDetails(status.assignments[0], status.planets, lang).map(x=>x.status).join(' | ')}
        | Ends: ${new Date(status.assignments[0].expiration).toLocaleString()}` : ""
      }
    </div>
  </div>
  <div class="center card">
    ${display(planetTableRows(agg, focus, status, lang))}
    ${display(renderDefenses(defenses, lang))}
    </div>
  </div>
</div>

```js
const showWaypoints = view(Inputs.toggle({label:"Show routes", value:false}))
const waypoints = status.planets.flatMap(x => x.waypoints.map(y => ({from:x.position, to:status.planets[y].position})));
const attacks = status.planets.flatMap(x => x.attacking.map(y => ({from:x.position, to:status.planets[y].position})));
```

<div class="grid grid-cols-4" style="grid-auto-rows: auto;">
  <div id="map-container" class="card grid-colspan-2 grid-rowspan-2">
    <div id="map">
    <h2>&nbsp;</h2>
    <img src="./data/sector_map.svg">
    </div>
    <div>${resize((width) => Plot.plot({
        width: width,
        title: "The Galactic War",
        aspectRatio: 1,
        height: width,
        projection: {type: "reflect-y", domain: {type: "MultiPoint", coordinates: [[100,-100],[100,100],[-100,100],[-100,-100]]}},
        marks: [
          Plot.dot(status.planets, {
            x: p => p.position.x,
            y: p => p.position.y, 
            r: width/150, 
            stroke: p => getColor(p.initial_owner),
            fill: p => getColor(p.current_owner), 
            strokeWidth: width/220,
            opacity: p => (active.includes(p.index) ? 1.0 : 0.25),
          }),
          showWaypoints ? null : Plot.arrow(attacks, {
            x1: p => p.from.x,
            y1: p => p.from.y,
            x2: p => p.to.x,
            y2: p => p.to.y,
            bend: true,
            inset: width/110,
            strokeWidth: width/440,
          }),
          showWaypoints ? Plot.link(waypoints, {
            x1: p => p.from.x,
            y1: p => p.from.y,
            x2: p => p.to.x,
            y2: p => p.to.y,
            inset: width/110,
            strokeWidth: width/880,
          }) : null,
          Plot.rect(status.active, {
            x1: p => p.planet.position.x-(width/440),
            y1: p => p.planet.position.y-(width/220),
            x2: p => p.planet.position.x+(width/440),
            y2: p => p.planet.position.y-(width/220)+1,
            stroke: "black",
            fill: p => getColor(getDefender(status, p.planet.index))
          }),
          Plot.rect(status.active, {
            x1: p => p.planet.position.x-(width/440),
            y1: p => p.planet.position.y-(width/220),
            x2: p => (p.planet.position.x-(width/440))+((width/220)*(getLiberation(p.planet.index, status, defenses)/100)),
            y2: p => p.planet.position.y-(width/220)+1,
            stroke: "black",
            fill: p => getColor('Humans')
          }),
          Plot.tip(status.planets, Plot.pointer({
            x: p => p.position.x, 
            y: p => p.position.y,
            title: p => [`${p.name[lang]}\n`, `Liberation: ${getLiberation(p.index, status, defenses).toFixed(2)}%`, `Players: ${p.statistics.player_count}`].join("\n"), fontSize: 20})
          ),
          factionLegend(['Humans', 'Terminids', 'Automaton', 'Illuminate'], {r:width/150, strokeWidth:width/220, width}),
        ],
        tip: true,
      }))
    }</div>
  </div>
  <div class="card grid-colspan-1" style="padding:1rem;">
  ${resize((width) => twoDayPlanetAttack(width, agg, focus[0][0], status.planets[focus[0][0]], lang))}
  </div>
  <div class="card grid-colspan-1">${resize((width) => twoDayPlanetAttack(width, agg, focus[1][0], status.planets[focus[1][0]], lang))}</div>
  <div class="card grid-colspan-1">${resize((width) => twoDayPlanetAttack(width, agg, focus[2][0], status.planets[focus[2][0]], lang))}</div>
  <div class="card grid-colspan-1">${resize((width) => twoDayPlanetAttack(width, agg, focus[3][0], status.planets[focus[3][0]], lang))}</div>
</div>

## History

```js
const v1 = x => x.players;
const v2 = x => x.impact;
const y2 = d3.scaleLinear(d3.extent(agg, v2), [0, d3.max(agg, v1)]);
const weekAgo = new Date(Date.now() - (60_000*60*24*7));
const filterLastWeek = x => (new Date(x.timestamp) > weekAgo);
```

### Player History

<div class="grid grid-cols-1">
  <div class="card">${
    resize((width) => Plot.plot({
      width:width,
      x: {domain:[weekAgo, Date.now()]},
      y: {axis: "left", label: "Players"},
      marks: [
        Plot.axisY(y2.ticks(), {color:"steelblue", anchor:"right", label: "Impact Multiplier", y: y2, tickFormat: y2.tickFormat()}),
        Plot.ruleY([0]),
        Plot.lineY(agg, {filter: filterLastWeek, x: "timestamp", y: "players", tip:"x", stroke: "red", strokeWidth: 4}),
        Plot.lineY(agg, Plot.mapY(D => D.map(y2), {filter: filterLastWeek, x: "timestamp", y: "impact", stroke: "steelblue"}))
      ]
    }))
  }</div>
</div>
