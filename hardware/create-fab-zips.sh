#!/bin/bash
# Little utility to pack stuff ready for osh-park.
# Choose 'export with proper filenames' in KiCad.
# Also choose to merge drill files in drill dialog.
# (this whole creating fab files from UI in KiCad drives me up the wall!)
##

if [ $# -ne 2 ] ; then
    echo "usage: $0 <gerber-dir> <project-name>"
    echo "Packs files together usable for OSH-Park"
    exit 1
fi

INDIR=$1
PROJECT=$2

IF=$INDIR/${PROJECT}

# for osh park
zip ${PROJECT}-board-fab.zip ${IF}*.{gbr,gbl,gtl,gbs,gts,gto,gbo,drl}

# for osh stencil
zip stencil-fab.zip ${IF}*.{gbr,gtp}
