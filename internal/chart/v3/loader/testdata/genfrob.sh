#!/bin/sh

# Pack the albatross chart into the mariner chart.
echo "Packing albatross into mariner"
tar -zcvf mariner/charts/albatross-0.1.0.tgz albatross

echo "Packing mariner into frobnitz"
tar -zcvf frobnitz/charts/mariner-4.3.2.tgz mariner
cp frobnitz/charts/mariner-4.3.2.tgz frobnitz_backslash/charts/
cp frobnitz/charts/mariner-4.3.2.tgz frobnitz_with_bom/charts/
cp frobnitz/charts/mariner-4.3.2.tgz frobnitz_with_dev_null/charts/
cp frobnitz/charts/mariner-4.3.2.tgz frobnitz_with_symlink/charts/

# Pack the frobnitz chart.
echo "Packing frobnitz"
tar --exclude=ignore/* -zcvf frobnitz-1.2.3.tgz frobnitz
tar --exclude=ignore/* -zcvf frobnitz_backslash-1.2.3.tgz frobnitz_backslash
tar --exclude=ignore/* -zcvf frobnitz_with_bom.tgz frobnitz_with_bom
