var satsumaApp = angular.module('satsuma', [ 'ngUpload' ]);


satsumaApp.directive('g+signin', function() {
	return {
		restrict: 'E',
		template: '<span></span>',
		replace: true,
		link: function(scope, element, attrs) {

			// Set class.
			attrs.$set('class', 'g-signin');

			attrs.$set('data-clientid', attrs.clientid + '.apps.googleusercontent.com');

			// Some default values, based on prior versions of this directive
			var defaults = {
				callback: 'signinCallback',
				cookiepolicy: 'single_host_origin',
				requestvisibleactions: 'http://schemas.google.com/AddActivity',
				scope: 'https://www.googleapis.com/auth/plus.login https://www.googleapis.com/auth/userinfo.email',
				width: 'wide'
			};

			// Provide default values if not explicitly set
			angular.forEach(Object.getOwnPropertyNames(defaults), function(propName) {
				if (!attrs.hasOwnProperty('data-' + propName)) {
					attrs.$set('data-' + propName, defaults[propName]);
				}
			});

			// Asynchronously load the G+ SDK.
			(function() {
				var po = document.createElement('script');
				po.type = 'text/javascript';
				po.async = true;
				po.src = 'https://apis.google.com/js/client:plusone.js';
				var s = document.getElementsByTagName('script')[0];
				s.parentNode.insertBefore(po, s);
			})();
		}
	};
});

satsumaApp.config(['$routeProvider', '$locationProvider', function($routeProvider, $locationProvider) {
		// TODO: configure routes
		$locationProvider.html5Mode(true);

		$routeProvider.when('/', { controller: 'MainCtrl' });

		$routeProvider.otherwise({ redirectTo: '/' });
	}
]);

satsumaApp.controller('MainCtrl', ['$scope', '$http', function($scope, $http) {

	$scope.loggedIn = false;
	$scope.error = null;

	window.signinCallback = function(authData) {
		$scope.error = null;
		console.log(authData);
		if (authData['access_token']) {
			$http.post('/api/connect', authData['code']).
			success(function(data, status, headers, config) {
				$scope.loggedIn = true;
			}).
			error(function(data, status, headers, config) {
				$scope.error = "Signing in failed. Please try again later.";
			});
		} else if (authData['error']) {
			$scope.error = "Signing in failed (" + authData['error'] + ").";
		}
		$scope.$apply();
	};

	$scope.closeError = function() {
		$scope.error = null;
	};

	$scope.signOut = function() {
		$http.post('/api/disconnect').
		success(function(data, status, headers, config) {
			$scope.loggedIn = false;
		}).
		error(function(data, status, headers, config) {
			$scope.loggedIn = false;
			console.log('disconnect failed: ' + data);
		});
	};

	$scope.uploadComplete = function(content, completed) {
		if (completed) {
			$scope.hideUpload();
		}
	};

	$scope.hideUpload = function() {
		$scope.showUpload = false;
		$('#upload_form').get(0).reset();
	};

	$scope.openUpload = function() {
		$scope.showUpload = true;
	};
}]);
