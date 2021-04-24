package main

var tmpl = `<!doctype html>
<html>

<head>
    <title>Electricity load / production forecast</title>
    <meta charset="utf-8" />
    <script src="https://cdnjs.cloudflare.com/ajax/libs/Chart.js/3.1.1/chart.min.js"
        integrity="sha512-BqNYFBAzGfZDnIWSAEGZSD/QFKeVxms2dIBPfw11gZubWwKUjEgmFUtUls8vZ6xTRZN/jaXGHD/ZaxD9+fDo0A=="
        crossorigin="anonymous"></script>
    <script
        src="https://cdnjs.cloudflare.com/ajax/libs/chartjs-plugin-annotation/1.0.0/chartjs-plugin-annotation.min.js"
        integrity="sha512-JP7f/zE4RT2Y3SG5zOmxsrPIy1MJst7hB6HoZtqDFia+oI4v3goB/Zt+A5jMR+DxdRFF075Y5Hc5z6wCGDW3uw=="
        crossorigin="anonymous"></script>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/luxon/1.26.0/luxon.min.js"
        integrity="sha512-cYjGoxnM2sxryDRpKhwi8uTszEw2zufuDVz1dWlm1+wlvwnhQ4wu00BokHH4IKlogGJNL+2a2GYvHN+xaoUxjQ=="
        crossorigin="anonymous"></script>
    <script src="https://cdn.jsdelivr.net/npm/chartjs-adapter-luxon@1.0.0/dist/chartjs-adapter-luxon.min.js"
        integrity="sha256-q8w2Mgq36OwAFKLLbdSi+aCHAb6WJhIswZ7N6k+xsf0=" crossorigin="anonymous"></script>
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

        window.onload = function () {

            const data = {
                datasets: [
                    {
                        label: 'Renewables total',
                        yAxisID: 'MW',
                        backgroundColor: 'rgba(75, 192, 192, 0.2)',
                        fill: true,
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
                        backgroundColor: "maroon",
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
                        backgroundColor: "yellow",
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
                        backgroundColor: "blue",
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
                        backgroundColor: 'lightblue',
                        fill: false,
                        borderColor: 'lightblue',
                        pointBackgroundColor: 'lightblue',
                        pointRadius: 2,
                        data: forecastWindOffShore,
                        type: 'line',
                        order: 3
                    },
                ],
            };

            const config = {
                type: 'line',
                data: data,
                options: {
                    plugins: {
                        title: {
                            text: 'Electricity load / production forecast',
                            display: true
                        },
                        annotation: {
                            annotations: [
                                {
                                    type: "line",
                                    drawTime: 'afterDatasetsDraw',
                                    mode: "vertical",
                                    scaleID: "x",
                                    value: luxon.DateTime.now().toISO(),
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
                        tooltip: {
                            callbacks: {
                                label: function (tooltipItem) {
                                    if (tooltipItem.datasetIndex == 1) {
                                        let addInfo = ""
                                        if (typeof forecastRenewablesPercent[tooltipItem.dataIndex] != 'undefined') {
                                            addInfo = " (" + forecastRenewablesPercent[tooltipItem.dataIndex].y + "% renewables)"
                                        }
                                        return Number(tooltipItem.formattedValue) + " MW" + addInfo;
                                    } else {
                                        if (tooltipItem.datasetIndex == 2) {
                                            return Number(tooltipItem.formattedValue) + "% renewables";
                                        } else {
                                            return data.datasets[tooltipItem.datasetIndex].label + ": " + Number(tooltipItem.formattedValue) + " MW";
                                        }
                                    }
                                },
                            }
                        },
                    },
                    responsive: true,
                    legend: {
                        position: 'top',
                    },
                    scales: {
                        x: {
                            type: 'time',
                            title: {
                                display: true,
                                text: 'time'
                            },
                            time: {
                                tooltipFormat: 'fff',
                                displayFormats: {
                                    hour: 'MMM d / HH:mm',
                                    minute: 'MMM d / HH:mm'
                                }
                            }
                        },
                        MW: {
                            id: 'MW',
                            type: 'linear',
                            position: 'left',
                            beginAtZero: true,
                            title: {
                                display: true,
                                text: "MW"
                            }
                        },
                        percent: {
                            id: 'percent',
                            type: 'linear',
                            position: 'right',
                            beginAtZero: true,
                            max: 100,
                            title: {
                                display: true,
                                text: "%"
                            }
                        }
                    },
                },
            };

            var ctx = document.getElementById('canvas').getContext('2d');
            var myChart = new Chart(ctx, config);

            var colors = []
            for (var i = 0; i < forecastLoad.length; i++) {
                let color = "green";
                if (i < forecastRenewablesPercent.length) {
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
                } else {
                    color = "rgb(210, 210, 210)"
                }

                colors.push(color);
            }
            config.data.datasets[1].backgroundColor = colors
            myChart.update();
        };
    </script>
</body>

</html>`
