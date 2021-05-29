#!/bin/sh

ssh lbsa mkdir -p /home/admin/molehill
rsync -aP . lbsa:/home/admin/molehill
