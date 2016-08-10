#!/bin/sh

# Pack the albatross chart into the mariner chart.
echo "Packing albatross into mariner"
tar -zcvf mariner/charts/albatross-0.1.0.tgz albatross

echo "Packing mariner into frobnitz"
tar -zcvf frobnitz/charts/mariner-4.3.2.tgz mariner

# Pack the frobnitz chart.
echo "Packing frobnitz"
tar --exclude=ignore/* -zcvf frobnitz-1.2.3.tgz frobnitz
