import * as Plot from "npm:@observablehq/plot";
import {scaleLinear, extent, max} from "npm:d3";
import {html} from "npm:htl";

export function twoDayPlanetAttack(width, agg, planetIdx, currentStatus){
    const v1 = x => x.attacks[planetIdx].players;
    const v2 = x => x.attacks[planetIdx].liberation;
    const y2 = scaleLinear([0, 100], [0, max(agg, v1)]);
    const start = new Date(Date.now() - (60_000*60*24*2));
    const first = agg.findIndex(x => (new Date(x.timestamp) > start));
    agg = agg.slice(first);
    return Plot.plot({
        title: currentStatus.planet.name,
        subtitle: getSubtitle(agg, planetIdx, currentStatus),
        width:width,
        x: {domain:[start, Date.now()]},
        y: {axis: "left", label: "Players"},
        marks: [
        Plot.axisY(y2.ticks(), {color:"#79E0FF", anchor:"right", label: "Liberation", y: y2, tickFormat: y2.tickFormat()}),
        Plot.ruleY([0]),
        Plot.lineY(agg, {x: "timestamp", y: v1, tip:"x", stroke: "red", strokeWidth: 2}),
        Plot.lineY(agg, Plot.mapY(D => D.map(y2), {x: "timestamp", y: v2, stroke: "#79E0FF"}))
        ]
    });
}

const secondsPerHour = 60 * 60;

function getRegen(planetStatus) {
    return (((planetStatus.regen_per_second * secondsPerHour) / planetStatus.planet.max_health) * 100.0).toFixed(2);
}

function calculateTrend(agg, planetIdx, planetStatus) {
    const SMOOTHING = 3;
    const liberation = x => x.attacks[planetIdx].liberation;
    const time = x => new Date(x.timestamp);
    let diffs = 0;
    let lastTime = new Date(agg[agg.length-SMOOTHING].timestamp);
    let lastLib = liberation(agg[agg.length-SMOOTHING]);
    for(let i=agg.length-(SMOOTHING-1); i < agg.length; i++){
        let timeDiff = time(agg[i]) - lastTime;
        lastTime = time(agg[i]);
        let libDiff = liberation(agg[i]) - lastLib;
        lastLib = liberation(agg[i]);
        diffs += libDiff/timeDiff; // delta Liberation / delta milliseconds
    }
    return (diffs/(SMOOTHING-1))*1000 * secondsPerHour;
}

function getResult(trend, planetStatus) {
    //TODO: this needs to be reworked for timed events like defenses
    if(Math.abs(trend) < 1e-3) {
        return "NO CHANGE";
    }
    let dividend = -planetStatus.liberation;
    let result = "LOSS";
    if(trend > 0) {
        dividend = 100.0 - planetStatus.liberation;
        result = "CAPTURE";
    }
    let hours = dividend / trend;
    return `${result} in ${formatHoursMinutes(hours * 60)}`;
}

function formatHoursMinutes(minutes) {
    return `${(minutes/60).toFixed()}h${((minutes%60).toFixed()+"").padStart(2, '0')}m`;
}

function getSubtitle(agg, planetIdx, planetStatus) {
    let current = calculateTrend(agg, planetIdx, planetStatus);
    return `Current regen: ${getRegen(planetStatus)}%/hr | Recent net trend: ${current.toFixed(2)}%/hr | Estimated result: ${getResult(current, planetStatus)}`;
}

export function planetTableRows(agg, recentAttacks, status) {
    let rows = [];
    for(const planet of recentAttacks){
        const planetIdx = planet[0];
        const planetStatus = status.planet_status[planetIdx];
        let current = calculateTrend(agg, planetIdx, planetStatus).toFixed(2);
        let regen = getRegen(planetStatus);
        let result = getResult(current, planetStatus);
        rows.push(html`<tr>
        <td>${planetStatus.planet.name}</td>
        <td>${planetStatus.players}</td>
        <td>${planetStatus.liberation.toFixed(2)}%</td>
        <td>${regen}%/hr</td>
        <td>${current}%/hr</td>
        <td>${result}</td></tr>`);
    }
    return html`<table><thead>
    <th>Planet</th>
    <th>Players</th>
    <th>Liberation</th>
    <th>Regen</th>
    <th>Recent Liberation Rate</th>
    <th>Estimated Result</th>
  </thead><tbody>${rows}</tbody></table>`;
}