package main

var tmpl = `<!doctype html>
<html>

<head>
    <title>Electricity load / production forecast</title>
    <meta charset="utf-8" />
    <script src="https://cdnjs.cloudflare.com/ajax/libs/moment.js/2.27.0/moment-with-locales.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/chart.js@2.9.3"></script>
    <script
        src="https://cdnjs.cloudflare.com/ajax/libs/chartjs-plugin-annotation/0.5.7/chartjs-plugin-annotation.min.js"
        integrity="sha512-9hzM/Gfa9KP1hSBlq3/zyNF/dfbcjAYwUTBWYX+xi8fzfAPHL3ILwS1ci0CTVeuXTGkRAWgRMZZwtSNV7P+nfw=="
        crossorigin="anonymous">
    </script>
    <script>
    	var forecastLoad = {{.ForecastLoad}}
	    var forecastSolar = {{.ForecastSolar}}
	    var forecastWindOnShore = {{.ForecastWindOnShore}}
	    var forecastWindOffShore = {{.ForecastWindOffShore}}
	    var forecastRenewables = {{.ForecastRenewables}}
	    var forecastRenewablesPercent = {{.ForecastRenewablesPercent}}
    </script>
    <style>
        canvas {
            -moz-user-select: none;
            -webkit-user-select: none;
            -ms-user-select: none;
        }
    </style>
</head>


<body>
    <div id="container" style="width: 100%;">
        <canvas id="canvas"></canvas>
    </div>
    <script>
        'use strict';

        console.log(moment("2020-09-04T22:00:00Z").format());

        window.chartColors = {
            red: 'rgb(255, 99, 132)',
            orange: 'rgb(255, 159, 64)',
            yellow: 'rgb(255, 205, 86)',
            green: 'rgb(75, 192, 192)',
            blue: 'rgb(54, 162, 235)',
            purple: 'rgb(153, 102, 255)',
            grey: 'rgb(201, 203, 207)'
        };
        var color = Chart.helpers.color;

        window.onload = function () {
            var ctx = document.getElementById('canvas').getContext('2d');
            window.myBar = new Chart(ctx, {
                type: 'bar',
                data: {
                    datasets: [
                        {
                            label: 'Renewables total',
                            yAxisID: 'MW',
                            backgroundColor: color(window.chartColors.green).alpha(0.5).rgbString(),
                            fill: false,
                            borderColor: "green",
                            pointBackgroundColor: "green",
                            pointRadius: 2,
                            data: forecastRenewables,
                            type: 'line',
                            order: 4
                        },
                        {
                            label: 'Load total',
                            yAxisID: 'MW',
                            pointRadius: 5,
                            data: forecastLoad,
                            type: 'bar',
                            order: 6
                        },
                        {
                            label: '% renewables',
                            yAxisID: 'percent',
                            fill: false,
                            borderColor: "maroon",
                            pointBackgroundColor: "maroon",
                            pointRadius: 2,
                            data: forecastRenewablesPercent,
                            type: 'line',
                            order: 5
                        },
                        {
                            label: 'Solar',
                            yAxisID: 'MW',
                            fill: false,
                            borderColor: "yellow",
                            pointBackgroundColor: "yellow",
                            pointRadius: 2,
                            data: forecastSolar,
                            type: 'line',
                            order: 1
                        },
                        {
                            label: 'Wind onshore',
                            yAxisID: 'MW',
                            fill: false,
                            borderColor: "blue",
                            pointBackgroundColor: "blue",
                            pointRadius: 2,
                            data: forecastWindOnShore,
                            type: 'line',
                            order: 2
                        },
                        {
                            label: 'Wind offshore',
                            yAxisID: 'MW',
                            backgroundColor: color(window.chartColors.lightblue).alpha(0.5).rgbString(),
                            fill: false,
                            borderColor: "lightblue",
                            pointBackgroundColor: "lightblue",
                            pointRadius: 2,
                            data: forecastWindOffShore,
                            type: 'line',
                            order: 3
                        },
                    ],
                },
                options: {
                    annotation: {
                        annotations: [
                            {
                                type: "line",
                                drawTime: 'afterDatasetsDraw',
                                mode: "vertical",
                                scaleID: "x-axis-0",
                                value: moment().format("YYYY-MM-DD HH:mm:ss"),
                                borderColor: "red",
                                borderWidth: 3,
                                label: {
                                    content: "now",
                                    enabled: true,
                                    position: "top"
                                }
                            }
                        ]
                    },
                    tooltips: {
                        callbacks: {
                            label: function (tooltipItem, data) {
                                if (tooltipItem.datasetIndex == 1) {
                                    let addInfo = ""
                                    if (typeof forecastRenewablesPercent[tooltipItem.index] != 'undefined') {
                                        addInfo = " (" + forecastRenewablesPercent[tooltipItem.index].y + "% renewables)"
                                    }
                                    return Number(tooltipItem.yLabel) + " MW" + addInfo;
                                } else {
                                    if (tooltipItem.datasetIndex == 2) {
                                        return Number(tooltipItem.yLabel) + "% renewables";
                                    } else {
                                        return data.datasets[tooltipItem.datasetIndex].label + ": " + Number(tooltipItem.yLabel) + " MW";
                                    }
                                }
                            },
                            //title: (items, data) => console.log(items[0].xLabel),
                        }
                    },
                    responsive: true,
                    legend: {
                        position: 'top',
                    },
                    title: {
                        display: true,
                        text: 'Electricity load / production forecast'
                    },
                    scales: {
                        xAxes: [{
                            yAxisID: 'time',
                            type: 'time',
                            scaleLabel: {
                                display: true,
                                labelString: "time"
                            },
                            time: {
                                tooltipFormat: 'lll',
                                displayFormats: {
                                    hour: 'MMM D / h:mm a',
                                    minute: 'MMM D / h:mm a'
                                }
                            }
                        }],
                        yAxes: [{
                            id: 'MW',
                            type: 'linear',
                            position: 'left',
                            ticks: {
                                beginAtZero: true
                            },
                            scaleLabel: {
                                display: true,
                                labelString: "MW"
                            }
                        },
                        {
                            id: 'percent',
                            type: 'linear',
                            position: 'right',
                            ticks: {
                                beginAtZero: true,
                                max: 100
                            },
                            scaleLabel: {
                                display: true,
                                labelString: "%"
                            }
                        }]
                    }
                }
            });

            var x = []
            for (var i = 0; i < forecastRenewablesPercent.length; i++) {
                color = "green";
                // You can check for bars[i].value and put your conditions here
                if (forecastRenewablesPercent[i].y > 50) {
                    color = "rgba(0, 128, 0, 0.6)";
                }
                else if (forecastRenewablesPercent[i].y < 30) {
                    color = "rgba(255, 0, 0, 0.6)";

                }
                else {
                    color = "rgba(255, 165, 0, 0.6)";
                }

                x.push(color);
            }
            window.myBar.data.datasets[1].backgroundColor = x
            window.myBar.update();
        };
    </script>
</body>
</html>`
