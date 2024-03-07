import * as Plot from "npm:@observablehq/plot";
import {scaleLinear, extent, max} from "npm:d3";

export function twoDayPlanetAttack(width, agg, planetIdx, name){
    const v1 = x => x.attacks[planetIdx].players;
    const v2 = x => x.attacks[planetIdx].liberation;
    const y2 = scaleLinear([0, 100], [0, max(agg, v1)]);
    const start = new Date(Date.now() - (60_000*60*24*2));
    const first = agg.findIndex(x => (new Date(x.timestamp) > start));
    agg = agg.slice(first);
    return Plot.plot({
        title: name,
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