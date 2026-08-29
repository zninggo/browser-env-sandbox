#!/bin/bash
cd /mnt/f/projects/browser-env-sandbox
echo "=== 20-seed diversity test ==="
for s in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
  gpu=$(./bes fingerprint --browser chrome --os windows --seed $s 2>&1 | grep "^GPU:" | cut -c1-50)
  chrome=$(./bes fingerprint --browser chrome --os windows --seed $s 2>&1 | grep "^UA:" | grep -oP "Chrome/\d+")
  tz=$(./bes fingerprint --browser chrome --os windows --seed $s 2>&1 | grep "^Timezone:" | cut -c11-)
  screen=$(./bes fingerprint --browser chrome --os windows --seed $s 2>&1 | grep "^Screen:" | grep -oP "width:\d+" | head -1)
  win=$(./bes fingerprint --browser chrome --os windows --seed $s 2>&1 | grep "^Window:" | cut -c1-25)
  echo "seed=$s gpu=${gpu:0:45} $chrome tz=$tz $screen $win"
done
