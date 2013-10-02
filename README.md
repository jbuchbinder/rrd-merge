# RRD-MERGE

[![Build Status](https://secure.travis-ci.org/jbuchbinder/rrd-merge.png)](http://travis-ci.org/jbuchbinder/rrd-merge)

* Homepage: https://github.com/jbuchbinder/rrd-merge
* Twitter: [@jbuchbinder](https://twitter.com/jbuchbinder)

## USAGE

```
rrd-merge [-debug] OLDFILE.rrd NEWFILE.rrd OUTPUTFILE.rrd
```

## BUILDING

This requires **rrdtool** be installed and executable on your PATH. It
was tested with 1.4.7, so your mileage may vary.

```
go build
```

