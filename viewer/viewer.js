// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

var selected_capture;
var selected_traces = {};
var trace_dygraph;

var PlotTraceData = function() {
    var max_samples = 0;
    for (t in selected_traces) {
        max_samples = Math.max(max_samples, selected_traces[t].length);
    }

    // convert traces data to a multidim array:
    // data = [
    //   [1, trace1_sample1, trace2_sample1],
    //   [2, trace1_sample2, trace2_sample2],
    //   [3, trace1_sample3, trace2_sample3],
    // ]
    var data = [];
    for (var i = 0; i < max_samples; i++) {
        var i_samples = [i, ];

        for (t in selected_traces) {
            if (i < selected_traces[t].length) {
                i_samples.push(selected_traces[t][i]);
            } else {
                i_samples.push(0);
            }
        }

        data.push(i_samples);
    }

    var labels = ["sample"].concat(Object.keys(selected_traces));
    trace_dygraph = new Dygraph(
        document.getElementById("trace_plot"),
        data, {
            legend: "always",
            animatedZooms: true,
            title: selected_capture + " power trace",
            labels: labels,
        });
};

var LoadTraceData = function(capture, trace) {
    $.ajax({
        url: "/data/" + capture + "/" + trace,
        method: "GET",
        dataType: "json",
        success: function(d) {
            selected_traces[trace] = d;
            PlotTraceData();
        }
    });
};

var LoadTraces = function(capture) {
    if (trace_dygraph) {
        trace_dygraph.destroy();
        trace_dygraph = null;
        selected_traces = {};
    }
    $.ajax({
        url: "/data/" + capture,
        method: "GET",
        dataType: "json",
        success: function(d) {
            // Automatically load the first trace.
            if (d.length > 0) {
                d[0]["Selected"] = true;
                LoadTraceData(selected_capture, d[0]["Id"]);
            }
            $("#traces").bootstrapTable("load", d);
        },
        error: function() {
            $("#traces").bootstrapTable("load", []);
        },
    });
}

var LoadCaptures = function(wait) {
    $.ajax({
        url: "/captures",
        method: "GET",
        data: {
            "wait": wait
        },
        dataType: "json",
        success: function(d) {
            $("#captures").empty();
            d.forEach(function(value, i) {
                $("#captures")
                    .append($("<li>").attr("class", "nav-item")
                        .append($("<a>").attr("class", "nav-link" + (value == selected_capture ? " active" : ""))
                            .attr('id', "cap_" + value)
                            .attr("href", "#" + value)
                            .append($("<span>").attr("data-feather", "file-text"))
                            .append(value)));
            })
            feather.replace();
            // Automatically load the first capture.
            if (!wait && d.length > 0) {
              selected_capture = d[0];
              $("#cap_" + selected_capture).addClass("active");
              LoadTraces(d[0]);
            }
            $.when().then(LoadCaptures(true));
        }
    });

    $("a.nav-link").click(function(event) {
        event.preventDefault();
        var url = $(this).attr("href");
        var new_selected_capture = url.substring(1);
        if (selected_capture != new_selected_capture) {
            $("#cap_" + selected_capture).removeClass("active");
            selected_capture = new_selected_capture;
            $("#cap_" + selected_capture).addClass("active");
            LoadTraces(selected_capture);
        } else if (trace_dygraph) {
            trace_dygraph.resetZoom();
        }
    });
};

$(document).ready(function() {
    "use strict"
    $("#traces").bootstrapTable({
        onClickRow: function(row, elm, field) {
            if (row.Selected) {
                delete selected_traces[row.Id];
                PlotTraceData();
            } else {
                LoadTraceData(selected_capture, row.Id);
            }
        },
    });
    feather.replace();
    LoadCaptures(false);
})
