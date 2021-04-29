#!/usr/local/bin/python3
# -*- coding: utf-8 -*-
from wetterdienst.provider.dwd.forecast import (
    DwdForecastDate,
    DwdMosmixRequest,
    DwdMosmixType,
)
from wetterdienst.util.cli import setup_logging
import pandas


def mosmix_example():
    request = DwdMosmixRequest(
        parameter=["ttt", "ff", "rad1h"],
        start_issue=DwdForecastDate.LATEST,  # automatically set if left empty
        mosmix_type=DwdMosmixType.SMALL,
        tidy=False,
        humanize=False,
    )

    stations = request.filter(
        # Hamburg inner city
        station_id=["P0489"], # "10007"],
    )
    response = next(stations.values.query())

    output_section("Metadata", response.stations.df)
    output_section("Forecasts", response.df)


def output_section(title, data):  # pragma: no cover
    print("-" * len(title))
    print(title)
    print("-" * len(title))
    print(data)
    print()


def main():
    pandas.set_option('display.max_rows', None)
    pandas.set_option('display.max_columns', None)
    pandas.set_option('display.width', None)
    pandas.set_option('display.max_colwidth', None)

    setup_logging()
    mosmix_example()


if __name__ == "__main__":
    main()
