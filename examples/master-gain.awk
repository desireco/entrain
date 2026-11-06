# Read FFmpeg's ebur128=peak=true summary. Return a fixed gain in dB
# By default target -24 LUFS with at least 3 dB of true-peak headroom.
BEGIN {
  if (!length(target)) target = -24
  if (!length(ceiling)) ceiling = -3
}
$1 == "I:" { loudness = $2 }
$1 == "Peak:" { peak = $2 }
END {
  if (loudness == "" || peak == "" || loudness == "-inf" || peak == "-inf") exit 1
  gain = target - loudness
  if (peak + gain > ceiling) gain = ceiling - peak
  printf "%.2f", gain
}
