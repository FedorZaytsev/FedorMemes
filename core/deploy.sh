#!/bin/sh
rsync -av ./ ../fedormemesprod --exclude={config.toml,fedormemes.db,fedormemes}
cd ../fedormemesprod
vgo build
#sudo service fedormemes restart
#sudo service fedormemes status
#cd -
