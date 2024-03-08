BUILD_CHANNEL?=local

appimage: clean-appimage
	cd etc/packaging/appimages && BUILD_CHANNEL=${BUILD_CHANNEL} appimage-builder --recipe viam-kuka-module-`uname -m`.yml
	if [ "${RELEASE_TYPE}" = "stable" ]; then \
		cd etc/packaging/appimages; \
		BUILD_CHANNEL=stable appimage-builder --recipe viam-kuka-module-`uname -m`.yml; \
	fi
	mkdir -p etc/packaging/appimages/deploy/
	mv etc/packaging/appimages/*.AppImage* etc/packaging/appimages/deploy/
	chmod 755 etc/packaging/appimages/deploy/*.AppImage

clean-appimage:
	rm -rf etc/packaging/appimages/AppDir 
	rm -rf etc/packaging/appimages/appimage-build 
	rm -rf etc/packaging/appimages/deploy