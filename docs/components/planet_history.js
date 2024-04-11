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
        title: currentStatus.name,
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

const SEC_PER_HOUR = 60 * 60;
const MSEC_PER_HOUR = SEC_PER_HOUR * 1000;

function getRegen(planetStatus) {
    return (((planetStatus.regen_per_second * SEC_PER_HOUR) / planetStatus.max_health) * 100.0).toFixed(2);
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
    return (diffs/(SMOOTHING-1))*MSEC_PER_HOUR;
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

function getDefenseResult(trend, liberation, timeRemaining){
    // We need to know trend, time remaining, and current progress
    const liberationNeeded = 100.0-liberation;
    const remainingHours = (timeRemaining / MSEC_PER_HOUR);
    const libPerHourNeeded = liberationNeeded / remainingHours;
    if(trend > libPerHourNeeded) {
        return `SUCCESS in ${formatHoursMinutes((liberationNeeded/trend) * 60)}`;
    }else {
        return "FAILURE";
    }
}

function formatHoursMinutes(minutes) {
    return `${Math.floor(minutes/60).toFixed()}h${((minutes%60).toFixed()+"").padStart(2, '0')}m`;
}

function getSubtitle(agg, planetIdx, planetStatus) {
    let current = calculateTrend(agg, planetIdx, planetStatus);
    let result = null;
    let regen = 0;
    if(planetStatus.result_str) {
        result = planetStatus.result_str;
    }else {
        result = `${getResult(current, planetStatus)}%/hr`;
    }
    if(planetStatus.regen_per_hour){
        regen = planetStatus.regen_per_hour;
    }else {
        getRegen(planetStatus);
    }
    return `Current regen: ${regen}%/hr | Recent net trend: ${current.toFixed(2)}%/hr | Estimated result: ${result}`;
}

export function planetTableRows(agg, recentAttacks, status, gameTime) {
    let rows = [];
    for(const planet of recentAttacks){
        const planetIdx = planet[0];
        const planetStatus = status.planets[planetIdx];
        let regen = getRegen(planetStatus);
        let current = calculateTrend(agg, planetIdx, planetStatus).toFixed(2);
        let result = getResult(current, planetStatus);
        let event = isDefense(status, planetIdx);
        if(event){
            let liberation = agg[agg.length-1]["attacks"][planetIdx].liberation;
            planetStatus.liberation = liberation;
            planetStatus['result_str'] = getDefenseResult(current, liberation, new Date(event.end_time) - gameTime);
            planetStatus['regen_per_hour'] = "N/A";
            rows.push(html`<tr>
            <td>${planetStatus.name}</td>
            <td>${planetStatus.players}</td>
            <td>${liberation.toFixed(2)}%</td>
            <td>N/A</td>
            <td>${current}%/hr</td>
            <td>${planetStatus.result_str}</td></tr>`);
        }else{
            planetStatus['result_str'] = result;
            planetStatus['regen_per_hour'] = regen;
            rows.push(html`<tr>
            <td>${planetStatus.name}</td>
            <td>${planetStatus.players}</td>
            <td>${planetStatus.liberation.toFixed(2)}%</td>
            <td>${regen}%/hr</td>
            <td>${current}%/hr</td>
            <td>${result}</td></tr>`);
        }
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

function isDefense(status, planetIdx) {
    for(let event of status.events) {
        if(event.planet.index == planetIdx){
            return event;
        }
    }
    return false;
}

export function renderDefenses(defenses, msSinceGameEpoch){
    let rows = [];
    for(let defense of defenses){
        rows.push(html`<div>
        <strong>🛡 ${defense.planet.name} is under attack! 🛡 </strong> <br>
        <p>
        Defense progress: ${(100* (1.0 - defense.health/defense.max_health)).toFixed(2)}%.
        This event ends at ${new Date(Date.now() + new Date(defense.end_time).getTime()-msSinceGameEpoch).toLocaleString()}. 
        Rate needed: ${
            (100*(defense.health / defense.max_health)/((new Date(defense.end_time).getTime()-msSinceGameEpoch)/(MSEC_PER_HOUR))).toFixed(2)
        }%/hr
        </p>
        </div>`);
    }
    return rows;
}

export function getDefender(status, index){
    let def = isDefense(status, index);
    if(def) {
        return def.faction;
    }
    return status.planets[index].current_owner;
}

export function renderAHTags(string){
    return string.replaceAll("</i>", "")
        .replaceAll("<i=1>","")
        .replaceAll("<i=3>", "");
}
